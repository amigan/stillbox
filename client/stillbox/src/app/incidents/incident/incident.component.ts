import { Component, inject, Input } from '@angular/core';
import { tap } from 'rxjs/operators';
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
import { IncidentsService } from '../incidents.service';
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
import {
  FixedPointPipe,
  TalkgroupPipe,
  TimePipe,
  DatePipe,
  DownloadURLPipe,
} from '../../calls/calls.service';
import { CallPlayerComponent } from '../../calls/player/call-player/call-player.component';
import { FmtDatePipe } from '../incidents.component';
import { MatMenuModule } from '@angular/material/menu';
import { Share } from '../../shares';
import { ShareService } from '../../share/share.service';
import { TalkgroupService } from '../../talkgroups/talkgroups.service';

export interface EditDialogData {
  incID: string;
  new: boolean;
}

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
    FixedPointPipe,
    TimePipe,
    DatePipe,
    TalkgroupPipe,
    DownloadURLPipe,
    CallPlayerComponent,
    FmtDatePipe,
    MatTableModule,
    MatMenuModule,
  ],
  templateUrl: './incident.component.html',
  styleUrl: './incident.component.scss',
})
export class IncidentComponent {
  incPrime = new Subject<IncidentRecord>();
  inc$!: Observable<IncidentRecord>;
  @Input() share?: Share;
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
    'duration',
  ];
  callsResult = new MatTableDataSource<IncidentCall>();
  selection = new SelectionModel<IncidentCall>(true, []);

  constructor(
    private route: ActivatedRoute,
    private incSvc: IncidentsService,
    private location: Location,
    private tgSvc: TalkgroupService,
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
    this.inc$ = merge(incOb, this.incPrime).pipe(
      tap((inc) => {
        if (inc && inc.calls) {
          this.callsResult.data = inc.calls;
        }
      }),
    );
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

  ngOnDestroy() {
    this.subscriptions.unsubscribe();
  }

  isAllSelected() {
    const numSelected = this.selection.selected.length;
    const numRows = this.callsResult.data.length;
    return numSelected === numRows;
  }

  masterToggle() {
    this.isAllSelected()
      ? this.selection.clear()
      : this.callsResult.data.forEach((row) => this.selection.select(row));
  }
}
