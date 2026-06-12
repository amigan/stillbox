import { Component, inject, ViewChild } from '@angular/core';
import { CommonModule, AsyncPipe } from '@angular/common';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { MatTableModule } from '@angular/material/table';
import {
  MatPaginator,
  MatPaginatorModule,
  PageEvent,
} from '@angular/material/paginator';
import { PrefsService } from '../prefs/prefs.service';
import { MatIconModule } from '@angular/material/icon';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { Observable, Subject, Subscription } from 'rxjs';
import { map, switchMap } from 'rxjs/operators';
import { CallsListParams, CallsService } from './calls.service';
import { CallRecord } from '../calls';

import { TalkgroupService } from '../talkgroups/talkgroups.service';
import { MatFormFieldModule } from '@angular/material/form-field';
import {
  FormControl,
  FormGroup,
  FormsModule,
  ReactiveFormsModule,
} from '@angular/forms';
import { MatInputModule } from '@angular/material/input';
import { debounceTime, skip } from 'rxjs/operators';
import { ToolbarContextService } from '../navigation/toolbar-context.service';
import { MatSelectModule } from '@angular/material/select';
import { MatMenuModule } from '@angular/material/menu';
import { MatDialog } from '@angular/material/dialog';
import {
  EditDialogData,
  IncidentEditDialogComponent,
} from '../incidents/incident/incident.component';
import {
  CallIncidentParams,
  IncidentsService,
} from '../incidents/incidents.service';
import { IncidentRecord } from '../incidents';
import {
  SelectIncidentDialogComponent,
  SelectIncidentDialogData,
} from '../incidents/select-incident-dialog/select-incident-dialog.component';
import { MatSnackBar, MatSnackBarModule } from '@angular/material/snack-bar';
import { ErrorsService } from '../errors/errors.service';
import { toSignal } from '@angular/core/rxjs-interop';
import { MatTooltipModule } from '@angular/material/tooltip';
import { PlayerService } from './player/player.service';
import {
  PER_PAGE_DEFAULT,
  CallsTableComponent,
} from './calls-table/calls-table.component';
import { MatButtonModule } from '@angular/material/button';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { ActivatedRoute, Router } from '@angular/router';
import { Params } from '@angular/router';

const DEBOUNCE_INTERVAL = 300;

const reqPageSize = 200;
@Component({
  selector: 'app-calls',
  imports: [
    MatIconModule,
    MatPaginatorModule,
    MatButtonToggleModule,
    MatCheckboxModule,
    MatTableModule,
    AsyncPipe,
    MatFormFieldModule,
    ReactiveFormsModule,
    FormsModule,
    MatInputModule,
    MatCheckboxModule,
    CommonModule,
    MatProgressSpinnerModule,
    MatProgressBarModule,
    MatButtonModule,
    MatSelectModule,
    MatMenuModule,
    MatTooltipModule,
    MatSnackBarModule,
    CallsTableComponent,
  ],
  templateUrl: './calls.component.html',
  styleUrl: './calls.component.scss',
})
export class CallsComponent {
  @ViewChild('callsTable') callsTable!: CallsTableComponent;
  callsResult = new Subject<Array<CallRecord>>();
  dialog = inject(MatDialog);
  private snackBar = inject(MatSnackBar);
  private errorsSvc = inject(ErrorsService);
  private route = inject(ActivatedRoute);
  /** When set (e.g. from incident "Add calls…"), target incident is pre-selected in the picker. */
  addToIncidentId = toSignal(
    this.route.queryParams.pipe(
      map((p) => (p['addToIncident'] as string | undefined) ?? null),
    ),
    { initialValue: this.route.snapshot.queryParams['addToIncident'] ?? null },
  );
  showTranscripts!: Observable<boolean>;
  currentSet!: CallRecord[];

  curPage = <PageEvent>{ pageIndex: 0, pageSize: PER_PAGE_DEFAULT };
  queryInProgress = false;
  currentServerPage = 0; // page is never 0, forces load

  perPage = PER_PAGE_DEFAULT;
  form = new FormGroup({
    start: new FormControl(this.lTime(new Date())),
    end: new FormControl(null),
    filter: new FormControl(''),
    sourceFilter: new FormControl(''),
    transcriptSearch: new FormControl<string | null>(null),
    duration: new FormControl(0),
    tagsAny: new FormControl<string[]>([]),
    tagsNot: new FormControl<string[]>([]),
    showTranscripts: new FormControl(false),
  });
  isLoading = true;
  transcriptFilter = toSignal(
    this.form.controls.transcriptSearch.valueChanges.pipe(
      debounceTime(DEBOUNCE_INTERVAL),
    ),
  );
  showTranscriptsControl = toSignal(
    this.form.controls.showTranscripts.valueChanges,
  );

  subscriptions = new Subscription();
  pageWindow = 0;
  fetchCalls = new Subject<CallsListParams>();
  private fromQueueOriginNav = false;

  constructor(
    private callsSvc: CallsService,
    private prefsSvc: PrefsService,
    public tcSvc: ToolbarContextService,
    public tgSvc: TalkgroupService,
    public incSvc: IncidentsService,
    public playerSvc: PlayerService,
    private router: Router,
  ) {
    this.tcSvc.showFilterButton();
    this.fromQueueOriginNav =
      this.router.getCurrentNavigation()?.extras?.state?.['fromQueueOrigin'] ===
      true;
  }

  private isFromQueueOrigin(): boolean {
    return this.fromQueueOriginNav;
  }

  /** Build query params from current form for queue-origin link (e.g. now-playing). */
  buildQueueOriginQueryParams(): Params {
    const f = this.form.controls;
    const q: Params = {};
    const start = f['start'].value;
    if (start) q['start'] = start;
    const end = f['end'].value;
    if (end) q['end'] = end;
    const filter = f['filter'].value;
    if (filter) q['filter'] = filter;
    const sourceFilter = f['sourceFilter'].value;
    if (sourceFilter) q['sourceFilter'] = sourceFilter;
    const transcriptSearch = f['transcriptSearch'].value;
    if (transcriptSearch) q['transcriptSearch'] = transcriptSearch;
    const duration = f['duration'].value;
    if (duration != null && duration > 0) q['duration'] = String(duration);
    const tagsAny = f['tagsAny'].value;
    if (tagsAny?.length) q['tagsAny'] = JSON.stringify(tagsAny);
    const tagsNot = f['tagsNot'].value;
    if (tagsNot?.length) q['tagsNot'] = JSON.stringify(tagsNot);
    if (f['showTranscripts'].value) q['showTranscripts'] = 'true';
    return q;
  }

  /** Patch form from URL query params (e.g. when opening link from now-playing). */
  patchFormFromQueryParams(params: Params): void {
    if (Object.keys(params).length === 0) return;
    const f = this.form.controls;
    if (params['start'] != null)
      f['start'].setValue(params['start'], { emitEvent: false });
    if (params['end'] != null)
      f['end'].setValue(params['end'], { emitEvent: false });
    if (params['filter'] != null)
      f['filter'].setValue(params['filter'], { emitEvent: false });
    if (params['sourceFilter'] != null)
      f['sourceFilter'].setValue(params['sourceFilter'], { emitEvent: false });
    if (params['transcriptSearch'] != null)
      f['transcriptSearch'].setValue(params['transcriptSearch'], {
        emitEvent: false,
      });
    if (params['duration'] != null) {
      const n = Number(params['duration']);
      if (!Number.isNaN(n)) f['duration'].setValue(n, { emitEvent: false });
    }
    if (params['tagsAny'] != null) {
      try {
        const a = JSON.parse(params['tagsAny']);
        if (Array.isArray(a)) f['tagsAny'].setValue(a, { emitEvent: false });
      } catch (_) {}
    }
    if (params['tagsNot'] != null) {
      try {
        const a = JSON.parse(params['tagsNot']);
        if (Array.isArray(a)) f['tagsNot'].setValue(a, { emitEvent: false });
      } catch (_) {}
    }
    if (params['showTranscripts'] === 'true')
      f['showTranscripts'].setValue(true, { emitEvent: false });
  }

  txSearchSet(): boolean {
    let tf = this.transcriptFilter();
    let stt = this.showTranscriptsControl();
    return (tf != null && tf.length > 0) || (stt != null && stt);
  }

  searchTGFilter(filt: string | null) {
    if (filt != null && filt != this.form.controls.filter.value) {
      this.form.controls['filter'].setValue(filt);
    }
  }

  searchSrcFilter(filt: string | null) {
    if (filt != null && filt != this.form.controls.sourceFilter.value) {
      this.form.controls['sourceFilter'].setValue(filt);
    }
  }

  transcriptSearchString(v: string | null): string | null {
    if (v == ' ') {
      return '';
    }
    if (v != '' && v != null) {
      return v;
    }

    if (this.form.controls.showTranscripts.value) {
      return '';
    }

    return null;
  }

  buildParams(p: PageEvent, serverPage: number): CallsListParams {
    const par: CallsListParams = {
      start: new Date(this.form.controls['start'].value!),
      page: serverPage,
      perPage: reqPageSize,
      end:
        this.form.controls['end'].value != null
          ? new Date(this.form.controls['end'].value!)
          : null,
      tagsAny:
        (this.form.controls['tagsAny'].value?.length ?? 0 > 0)
          ? this.form.controls['tagsAny'].value
          : null,
      tagsNot:
        (this.form.controls['tagsNot'].value?.length ?? 0 > 0)
          ? this.form.controls['tagsNot'].value
          : null,
      dir: 'desc',
      tgFilter:
        this.form.controls['filter'].value != ''
          ? this.form.controls['filter'].value
          : null,
      sourceFilter:
        this.form.controls['sourceFilter'].value != ''
          ? this.form.controls['sourceFilter'].value
          : null,
      transcriptSearch: this.transcriptSearchString(
        this.form.controls['transcriptSearch'].value,
      ),
      atLeastSeconds:
        this.form.controls['duration'].value != null &&
        this.form.controls['duration'].value > 0
          ? this.form.controls['duration'].value
          : null,
    };

    return par;
  }

  lTime(now: Date): string {
    now.setDate(new Date().getDate() - 3);
    now.setMinutes(now.getMinutes() - now.getTimezoneOffset());
    return now.toISOString().slice(0, 16);
  }

  setPage(p: PageEvent, force?: boolean, dontClear?: boolean) {
    if (p.pageSize == 0) {
      p.pageSize = this.curPage.pageSize;
    }
    if (!dontClear && this.callsTable) {
      this.callsTable.selection?.clear();
    }
    this.curPage = p;
    if (p !== null && p!.pageSize != this.perPage) {
      this.perPage = p!.pageSize;
      this.prefsSvc.set('callsPerPage', p!.pageSize);
    }
    this.getCalls(p, force);
  }

  prevPage() {
    let p = this.curPage;
    if (p.pageIndex > 0) {
      p.pageIndex--;
    }
    this.setPage(p, false, true);
  }

  refresh() {
    this.callsTable.selection.clear();
    this.getCalls(this.curPage, true);
  }

  spinBar() {
    this.queryInProgress = true;
  }

  stopSpinBar() {
    this.queryInProgress = false;
  }

  getCalls(p: PageEvent, force?: boolean) {
    const pageStart = p.pageIndex * p.pageSize;
    const serverPage = Math.floor(pageStart / reqPageSize) + 1;
    this.pageWindow = pageStart % reqPageSize;
    if (serverPage == this.currentServerPage && !force && this.currentSet) {
      this.callsResult.next(
        this.callsResult
          ? this.currentSet.slice(this.pageWindow, this.pageWindow + p.pageSize)
          : [],
      );
    } else {
      this.spinBar();
      this.currentServerPage = serverPage;
      this.fetchCalls.next(this.buildParams(p, serverPage));
    }
  }

  zeroPage(): PageEvent {
    return <PageEvent>{
      pageIndex: 0,
      pageSize: this.curPage.pageSize,
    };
  }

  ngOnDestroy() {
    this.tcSvc.hideFilterButton();
    this.subscriptions.unsubscribe();
  }

  ngOnInit() {
    this.form.valueChanges
      .pipe(debounceTime(DEBOUNCE_INTERVAL))
      .subscribe(() => {
        this.currentServerPage = 0;
        this.setPage(this.zeroPage(), true);
        this.router.navigate([], {
          relativeTo: this.route,
          queryParams: this.buildQueueOriginQueryParams(),
          replaceUrl: true,
        });
      });

    this.subscriptions.add(
      this.prefsSvc.get('callsPerPage').subscribe((cpp) => {
        if (this.perPage == 0) {
          this.perPage = PER_PAGE_DEFAULT;
        }
        if (cpp && cpp != this.perPage) {
          this.perPage = cpp;

          this.setPage(<PageEvent>{
            pageIndex: 0,
            pageSize: cpp,
          });
        }
      }),
    );
    this.subscriptions.add(
      this.prefsSvc.get('calls.view.showSourceAlias').subscribe((v) => {
        if (v != true) {
          this.callsTable.columns = this.callsTable.columns.filter(
            (e) => e != 'talker',
          );
        }
      }),
    );
    this.showTranscripts = this.prefsSvc.get('calls.view.showTranscripts');
    this.subscriptions.add(
      this.fetchCalls
        .pipe(
          debounceTime(500),
          switchMap((params) => {
            return this.callsSvc.getCalls(params);
          }),
        )
        .subscribe((calls) => {
          this.isLoading = false;
          this.stopSpinBar();
          this.callsTable.count = calls.count;
          this.currentSet = calls.calls;
          let sliceStart = this.pageWindow;
          const fromQueueOrigin = this.isFromQueueOrigin();
          const playingId = this.playerSvc.playing()?.id;
          if (
            fromQueueOrigin &&
            playingId &&
            this.currentSet?.some((c) => c.id === playingId)
          ) {
            const playingIndex = this.currentSet.findIndex(
              (c) => c.id === playingId,
            );
            const pageIndex = Math.floor(playingIndex / this.perPage);
            this.curPage = {
              pageIndex,
              pageSize: this.perPage,
              length: this.currentSet.length,
            };
            sliceStart = pageIndex * this.perPage;
            this.pageWindow = sliceStart;
          }
          if (this.callsTable) {
            this.callsTable.callsTable.nativeElement.scrollIntoView(true);
          }
          this.callsResult.next(
            this.currentSet
              ? this.currentSet.slice(sliceStart, sliceStart + this.perPage)
              : [],
          );
          if (fromQueueOrigin && playingId) {
            setTimeout(() => {
              this.scrollToPlayingCall(playingId);
              this.fromQueueOriginNav = false;
            }, 150);
          }
        }),
    );
    this.subscriptions.add(
      this.callsResult.subscribe((cr) => {
        if (this.callsTable != undefined) {
          this.callsSvc.curLen = cr.length;
        }
        this.playerSvc.setQueue(cr, {
          type: 'calls',
          queryParams: this.buildQueueOriginQueryParams(),
        });
      }),
    );
    this.patchFormFromQueryParams(this.route.snapshot.queryParams);
    this.subscriptions.add(
      this.route.queryParams.pipe(skip(1)).subscribe((params) => {
        this.patchFormFromQueryParams(params);
        this.currentServerPage = 0;
        this.setPage(this.zeroPage(), true);
      }),
    );
    this.fetchCalls.next(
      this.buildParams(this.curPage, this.curPage.pageIndex),
    );
  }

  scrollToPlayingCall(callId: string): void {
    const el = this.callsTable?.callsTable?.nativeElement as HTMLElement;
    const row = el?.querySelector<HTMLElement>(
      `tr[data-call-id="${CSS.escape(callId)}"]`,
    );
    row?.scrollIntoView({ block: 'center', behavior: 'smooth' });
  }

  resetFilter() {
    this.form.reset();
    this.form.controls['start'].setValue(this.lTime(new Date()));
    this.form.controls['duration'].setValue(0);
  }

  private getSelectedCallIds(): string[] {
    return this.callsTable.selection.selected.map((s) => s.id);
  }

  private clearAddToIncidentQueryParam(): void {
    const qp = { ...this.route.snapshot.queryParams };
    if (qp['addToIncident'] == null) {
      return;
    }
    delete qp['addToIncident'];
    void this.router.navigate([], {
      relativeTo: this.route,
      queryParams: qp,
      replaceUrl: true,
    });
  }

  private showCallsAttachedSnackbar(incidentId: string, count: number): void {
    const msg =
      count === 1
        ? 'Added 1 call to the incident'
        : `Added ${count} calls to the incident`;
    this.snackBar
      .open(msg, 'View', { duration: 8000 })
      .onAction()
      .subscribe(() => {
        void this.router.navigate(['/incidents', incidentId]);
      });
  }

  private attachSelectionToIncident(incidentId: string): void {
    const ids = this.getSelectedCallIds();
    if (!incidentId || ids.length === 0) {
      return;
    }
    this.incSvc
      .addRemoveCalls(incidentId, <CallIncidentParams>{
        add: ids,
        notes: null,
        remove: null,
      })
      .subscribe({
        next: () => {
          this.callsTable.selection.clear();
          this.clearAddToIncidentQueryParam();
          this.showCallsAttachedSnackbar(incidentId, ids.length);
        },
        error: (err: unknown) => {
          this.errorsSvc.show(String(err));
        },
      });
  }

  addToNewInc(_ev: Event) {
    const ids = this.getSelectedCallIds();
    if (ids.length === 0) {
      return;
    }
    const dialogRef = this.dialog.open(IncidentEditDialogComponent, {
      data: <EditDialogData>{
        incID: '',
        new: true,
      },
    });
    dialogRef.afterClosed().subscribe((res: IncidentRecord | undefined) => {
      if (!res?.id) {
        return;
      }
      this.attachSelectionToIncident(res.id);
    });
  }

  addToExistingInc(_ev: Event) {
    const ids = this.getSelectedCallIds();
    if (ids.length === 0) {
      return;
    }
    const pre = this.route.snapshot.queryParams['addToIncident'];
    const dialogRef = this.dialog.open(SelectIncidentDialogComponent, {
      ...(pre
        ? {
            data: <SelectIncidentDialogData>{
              preselectedIncidentId: pre,
            },
          }
        : {}),
    });
    dialogRef.afterClosed().subscribe((res: string | null | undefined) => {
      if (!res) {
        return;
      }
      this.attachSelectionToIncident(res);
    });
  }
}
