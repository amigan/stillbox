import { Component, inject, Input, ViewChild } from '@angular/core';
import { switchMap, tap } from 'rxjs/operators';
import { CommonModule, Location } from '@angular/common';
import { BehaviorSubject, merge, Subject, Subscription } from 'rxjs';
import { Observable } from 'rxjs';
import {
  ReactiveFormsModule,
  FormGroup,
  FormControl,
  FormsModule,
} from '@angular/forms';
import { ActivatedRoute } from '@angular/router';
import { MatInputModule } from '@angular/material/input';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MatIconModule } from '@angular/material/icon';
import {
  CallIncidentParams,
  IncidentCallsParams,
  IncidentsService,
} from '../incidents.service';
import { IncidentCall, IncidentRecord } from '../../incidents';
import { MatCardModule } from '@angular/material/card';
import {
  MAT_DIALOG_DATA,
  MatDialog,
  MatDialogActions,
  MatDialogContent,
  MatDialogRef,
  MatDialogTitle,
} from '@angular/material/dialog';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MatButtonModule } from '@angular/material/button';
import { SelectionModel } from '@angular/cdk/collections';
import { MatTableDataSource, MatTableModule } from '@angular/material/table';
import { FmtDatePipe } from '../incidents.component';
import { MatMenuModule } from '@angular/material/menu';
import { Share } from '../../shares';
import { ShareService } from '../../share/share.service';
import { TalkgroupService } from '../../talkgroups/talkgroups.service';
import {
  CallsTableComponent,
  PER_PAGE_DEFAULT,
} from '../../calls/calls-table/calls-table.component';
import { PageEvent } from '@angular/material/paginator';
import { PlayerService } from '../../calls/player/player.service';
import { ErrorsService } from '../../errors/errors.service';
import { MarkdownModule } from 'ngx-markdown';
import { CallsService } from '../../calls/calls.service';
import { PrefsService } from '../../prefs/prefs.service';
import { MatProgressBarModule } from '@angular/material/progress-bar';

export interface EditDialogData {
  incID: string;
  new: boolean;
}

const reqPageSize = 200;

@Component({
  selector: 'app-incident-editor',
  imports: [
    MatIconModule,
    MatFormFieldModule,
    ReactiveFormsModule,
    FormsModule,
    MatInputModule,
    MatCheckboxModule,
    CommonModule,
    MatProgressSpinnerModule,
    MatDialogTitle,
    MatDialogActions,
    MatDialogContent,
    MatButtonModule,
  ],
  templateUrl: './incident-editor-dialog.component.html',
  styleUrl: './incident.component.scss',
})
export class IncidentEditDialogComponent {
  dialogRef = inject(MatDialogRef<IncidentEditDialogComponent>);
  data = inject<EditDialogData>(MAT_DIALOG_DATA);
  title = this.data.new ? 'New Incident' : 'Edit Incident';
  inc$!: Observable<IncidentRecord>;
  form = new FormGroup({
    name: new FormControl(''),
    start: new FormControl(),
    end: new FormControl(),
    description: new FormControl(''),
  });

  constructor(private incSvc: IncidentsService) {}

  ngOnInit() {
    if (!this.data.new) {
      this.inc$ = this.incSvc.getIncident(this.data.incID).pipe(
        tap((inc) => {
          this.form.patchValue(inc);
        }),
      );
    } else {
      this.inc$ = new BehaviorSubject(<IncidentRecord>{});
    }
  }

  save() {
    let resObs: Observable<IncidentRecord>;
    let ir: IncidentRecord = <IncidentRecord>{
      name: this.form.controls['name'].dirty
        ? this.form.controls['name'].value
        : null,
      startTime: this.form.controls['start'].dirty
        ? this.form.controls['start'].value
        : null,
      endTime: this.form.controls['end'].dirty
        ? this.form.controls['end'].value
        : null,
      description: this.form.controls['description'].dirty
        ? this.form.controls['description'].value
        : null,
    };
    if (this.data.new) {
      resObs = this.incSvc.createIncident(ir);
    } else {
      resObs = this.incSvc.updateIncident(this.data.incID, ir);
    }
    resObs.subscribe({
      next: (ok) => {
        this.dialogRef.close(ok);
      },
      error: (er) => {
        alert(er);
      },
    });
  }

  cancel() {
    this.dialogRef.close();
  }
}

@Component({
  selector: 'app-incident',
  imports: [
    CommonModule,
    ReactiveFormsModule,
    FormsModule,
    MatInputModule,
    MatFormFieldModule,
    MatCheckboxModule,
    MatIconModule,
    MatCardModule,
    FmtDatePipe,
    MatTableModule,
    MatMenuModule,
    CallsTableComponent,
    MarkdownModule,
    MatProgressSpinnerModule,
    MatProgressBarModule,
  ],
  templateUrl: './incident.component.html',
  styleUrl: './incident.component.scss',
})
export class IncidentComponent {
  incPrime = new Subject<IncidentRecord>();
  inc$!: Observable<IncidentRecord>;
  @Input() share?: Share;
  @ViewChild('callsTable') callsTable!: CallsTableComponent;
  subscriptions: Subscription = new Subscription();
  dialog = inject(MatDialog);
  incID!: string;
  columns = [
    'select',
    'play',
    'download',
    'date',
    'time',
    'system',
    'group',
    'talkgroup',
    'talker',
    'duration',
  ];
  callsResult = new Subject<Array<IncidentCall>>();
  queryInProgress = false;
  selection = new SelectionModel<IncidentCall>(true, []);
  curPage = <PageEvent>{ pageIndex: 0, pageSize: PER_PAGE_DEFAULT };
  perPage = PER_PAGE_DEFAULT;
  currentServerPage = 0; // page is never 0, forces load

  pageWindow = 0;
  fetchCalls = new Subject<IncidentCallsParams>();
  currentSet!: IncidentCall[];

  constructor(
    private route: ActivatedRoute,
    private incSvc: IncidentsService,
    private prefsSvc: PrefsService,
    private location: Location,
    private tgSvc: TalkgroupService,
    private playerSvc: PlayerService,
    private errorsSvc: ErrorsService,
    private callsSvc: CallsService,
  ) {}

  saveIncName(ev: Event) {}

  ngOnInit() {
    if (this.share) {
      this.tgSvc.setShare(this.share);
    }
    let incOb: Observable<IncidentRecord>;
    if (this.route.component === this.constructor) {
      // loaded by route
      this.incID = this.route.snapshot.paramMap.get('id')!;
      incOb = this.incSvc.getIncident(this.incID);
    } else {
      if (!this.share) {
        return;
      }
      this.incID = (this.share.sharedItem as IncidentRecord).id;
      incOb = new BehaviorSubject(this.share.sharedItem as IncidentRecord);
    }
    this.inc$ = merge(incOb, this.incPrime);
    this.fetchCalls
      .pipe(
        switchMap((params) => {
          return this.incSvc.getIncidentCalls(this.incID, params);
        }),
      )
      .subscribe((calls) => {
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
      });
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
    });
    this.callsResult.subscribe((cr) => {
      if (this.callsTable != undefined) {
        this.callsSvc.curLen = cr.length;
      }
      this.playerSvc.setQueue(cr);
    });
    this.fetchCalls.next(
      this.buildParams(this.curPage, this.curPage.pageIndex),
    );
  }

  buildParams(p: PageEvent, serverPage: number): IncidentCallsParams {
    const par: IncidentCallsParams = {
      page: serverPage,
      perPage: reqPageSize,
    };

    return par;
  }

  editIncident(incID: string) {
    const dialogRef = this.dialog.open(IncidentEditDialogComponent, {
      data: <EditDialogData>{
        incID: incID,
        new: false,
      },
    });

    dialogRef.afterClosed().subscribe(this.incPrime);
  }

  deleteIncident(incID: string) {
    if (confirm('Are you sure you want to delete this incident?')) {
      this.incSvc.deleteIncident(incID).subscribe({
        next: () => {
          this.location.back();
        },
        error: (err) => {
          alert(err);
        },
      });
    }
  }

  removeSelectedCalls() {
    this.incSvc
      .addRemoveCalls(this.incID, <CallIncidentParams>{
        remove: this.callsTable.selection.selected.map((call) => call.id),
      })
      .subscribe({
        next: () => {
          this.refresh();
        },
        error: (err) => {
          this.errorsSvc.show(err);
        },
      });
  }

  setPage(p: PageEvent, force?: boolean, dontClear?: boolean) {
    if (p.pageSize == 0) {
      p.pageSize = this.curPage.pageSize;
    }
    if (!dontClear) {
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

  spinBar() {
    this.queryInProgress = true;
  }

  stopSpinBar() {
    this.queryInProgress = false;
  }

  zeroPage(): PageEvent {
    return <PageEvent>{
      pageIndex: 0,
      pageSize: this.curPage.pageSize,
    };
  }

  ngOnDestroy() {
    this.subscriptions.unsubscribe();
  }
}
