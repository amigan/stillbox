import { Component, ElementRef, inject, ViewChild } from '@angular/core';
import { CommonModule, AsyncPipe } from '@angular/common';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MatTable, MatTableModule } from '@angular/material/table';
import {
  MatPaginator,
  MatPaginatorModule,
  PageEvent,
} from '@angular/material/paginator';
import { PrefsService } from '../prefs/prefs.service';
import { MatIconModule } from '@angular/material/icon';
import { SelectionModel } from '@angular/cdk/collections';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { BehaviorSubject, Subject, Subscription } from 'rxjs';
import { switchMap } from 'rxjs/operators';
import {
  CallsListParams,
  CallsService,
  DatePipe,
  DownloadURLPipe,
  FixedPointPipe,
  TalkgroupPipe,
  TimePipe,
} from './calls.service';
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
import { CallPlayerComponent } from './player/call-player/call-player.component';
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

const reqPageSize = 200;
@Component({
  selector: 'app-calls',
  imports: [
    MatIconModule,
    FixedPointPipe,
    TalkgroupPipe,
    TimePipe,
    DatePipe,
    MatPaginatorModule,
    MatTableModule,
    AsyncPipe,
    DownloadURLPipe,
    MatFormFieldModule,
    ReactiveFormsModule,
    FormsModule,
    MatInputModule,
    MatCheckboxModule,
    CommonModule,
    MatProgressSpinnerModule,
    MatSelectModule,
    CallPlayerComponent,
    MatMenuModule,
  ],
  templateUrl: './calls.component.html',
  styleUrl: './calls.component.scss',
})
export class CallsComponent {
  callsResult = new BehaviorSubject(new Array<CallRecord>(0));
  @ViewChild('paginator') paginator!: MatPaginator;
  @ViewChild('callsTable', { read: ElementRef }) callsTable!: ElementRef;
  count = 0;
  dialog = inject(MatDialog);
  page = 0;
  perPage = 25;
  pageSizeOptions = [25, 50, 75, 100, 200];
  columns = [
    'select',
    'play',
    'download',
    'date',
    'time',
    'system',
    'group',
    'talkgroup',
    'duration',
  ];
  curPage = <PageEvent>{ pageIndex: 0, pageSize: 0 };
  curLen = 0;
  currentSet!: CallRecord[];
  currentServerPage = 0; // page is never 0, forces load
  isLoading = true;

  selection = new SelectionModel<CallRecord>(true, []);

  form = new FormGroup({
    start: new FormControl(this.lTime(new Date())),
    end: new FormControl(null),
    filter: new FormControl(''),
    duration: new FormControl(0),
    tagsAny: new FormControl<string[]>([]),
    tagsNot: new FormControl<string[]>([]),
  });

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
  ) {
    this.tcSvc.showFilterButton();
  }

  isAllSelected() {
    const numSelected = this.selection.selected.length;
    const numRows = this.curLen;
    return numSelected === numRows;
  }

  searchFilter(filt: string | null) {
    if (filt) {
      this.form.controls['filter'].setValue(filt);
    }
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
      atLeastSeconds:
        this.form.controls['duration'].value != null &&
        this.form.controls['duration'].value > 0
          ? this.form.controls['duration'].value
          : null,
    };

    return par;
  }

  masterToggle() {
    this.isAllSelected()
      ? this.selection.clear()
      : this.callsResult.value.forEach((row) => this.selection.select(row));
  }

  lTime(now: Date): string {
    now.setDate(new Date().getDate() - 7);
    now.setMinutes(now.getMinutes() - now.getTimezoneOffset());
    return now.toISOString().slice(0, 16);
  }

  setPage(p: PageEvent, force?: boolean) {
    this.selection.clear();
    this.curPage = p;
    if (p && p!.pageSize != this.perPage) {
      this.perPage = p!.pageSize;
      this.prefsSvc.set('callsPerPage', p!.pageSize);
    }
    this.getCalls(p, force);
  }

  refresh() {
    this.selection.clear();
    this.getCalls(this.curPage, true);
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
    this.form.valueChanges.pipe(debounceTime(300)).subscribe(() => {
      this.currentServerPage = 0;
      this.setPage(this.zeroPage(), true);
    });
    this.subscriptions.add(
      this.prefsSvc.get('callsPerPage').subscribe((cpp) => {
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
      this.fetchCalls
        .pipe(
          debounceTime(500),
          switchMap((params) => {
            return this.callsSvc.getCalls(params);
          }),
        )
        .subscribe((calls) => {
          this.isLoading = false;
          this.count = calls.count;
          this.currentSet = calls.calls;
          if (this.callsTable) {
            this.callsTable.nativeElement.scrollIntoView(true);
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
        this.curLen = cr.length;
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
          add: this.selection.selected.map((s) => s.id),
        })
        .subscribe({
          next: () => {
            this.selection.clear();
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
          add: this.selection.selected.map((s, i, a) => {
            s.incidents++;
            return s.id;
          }),
        })
        .subscribe({
          next: () => {
            this.selection.clear();
          },
          error: (err) => {
            alert(err);
          },
        });
    });
  }
}
