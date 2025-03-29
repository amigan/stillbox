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
import { BehaviorSubject, Observable, Subject, Subscription } from 'rxjs';
import { switchMap } from 'rxjs/operators';
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
import { debounceTime } from 'rxjs/operators';
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
import { SelectIncidentDialogComponent } from '../incidents/select-incident-dialog/select-incident-dialog.component';
import { toSignal } from '@angular/core/rxjs-interop';
import { MatTooltipModule } from '@angular/material/tooltip';
import { PlayerService } from './player/player.service';
import {
  PER_PAGE_DEFAULT,
  CallsTableComponent,
} from './calls-table/calls-table.component';

const DEBOUNCE_INTERVAL = 300;

const reqPageSize = 200;
@Component({
  selector: 'app-calls',
  imports: [
    MatIconModule,
    MatPaginatorModule,
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
    MatSelectModule,
    MatMenuModule,
    MatTooltipModule,
    CallsTableComponent,
  ],
  templateUrl: './calls.component.html',
  styleUrl: './calls.component.scss',
})
export class CallsComponent {
  @ViewChild('callsTable') callsTable!: CallsTableComponent;
  callsResult = new BehaviorSubject(new Array<CallRecord>(0));
  dialog = inject(MatDialog);
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
  });
  isLoading = true;
  transcriptFilter = toSignal(
    this.form.controls.transcriptSearch.valueChanges.pipe(
      debounceTime(DEBOUNCE_INTERVAL),
    ),
  );

  subscriptions = new Subscription();
  pageWindow = 0;
  fetchCalls = new BehaviorSubject<CallsListParams>(
    this.buildParams(this.curPage, this.curPage.pageIndex),
  );

  constructor(
    private callsSvc: CallsService,
    private prefsSvc: PrefsService,
    public tcSvc: ToolbarContextService,
    public tgSvc: TalkgroupService,
    public incSvc: IncidentsService,
    public playerSvc: PlayerService,
  ) {
    this.tcSvc.showFilterButton();
  }

  txSearchSet(): boolean {
    let tf = this.transcriptFilter();
    return tf != null && tf.length > 0;
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
    if (v != '') {
      return v;
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
    if (!dontClear) {
      this.callsTable.selection.clear();
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
          if (this.callsTable) {
            this.callsTable.callsTable.nativeElement.scrollIntoView(true);
          }
          this.callsResult.next(
            this.currentSet
              ? this.currentSet.slice(
                  this.pageWindow,
                  this.pageWindow + this.perPage,
                )
              : [],
          );
        }),
    );
    this.subscriptions.add(
      this.callsResult.subscribe((cr) => {
        this.callsTable.curLen = cr.length;
        this.playerSvc.setQueue(cr);
      }),
    );
  }

  resetFilter() {
    this.form.reset();
    this.form.controls['start'].setValue(this.lTime(new Date()));
    this.form.controls['duration'].setValue(0);
  }

  addToNewInc(ev: Event) {
    const dialogRef = this.dialog.open(IncidentEditDialogComponent, {
      data: <EditDialogData>{
        incID: '',
        new: true,
      },
    });
    dialogRef.afterClosed().subscribe((res: IncidentRecord) => {
      this.incSvc
        .addRemoveCalls(res.id, <CallIncidentParams>{
          add: this.callsTable.selection.selected.map((s) => s.id),
        })
        .subscribe({
          next: () => {
            this.callsTable.selection.clear();
          },
          error: (err) => {
            alert(err);
          },
        });
    });
  }

  addToExistingInc(ev: Event) {
    const dialogRef = this.dialog.open(SelectIncidentDialogComponent);
    dialogRef.afterClosed().subscribe((res: string) => {
      if (!res) {
        return;
      }
      this.incSvc
        .addRemoveCalls(res, <CallIncidentParams>{
          add: this.callsTable.selection.selected.map((s, i, a) => {
            s.incidents++;
            return s.id;
          }),
        })
        .subscribe({
          next: () => {
            this.callsTable.selection.clear();
          },
          error: (err) => {
            alert(err);
          },
        });
    });
  }
}
