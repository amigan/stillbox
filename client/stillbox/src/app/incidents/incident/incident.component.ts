import { Component, computed, inject, ViewChild } from '@angular/core';
import { map, tap } from 'rxjs/operators';
import { CommonModule, DatePipe } from '@angular/common';
import { Subject, Subscription } from 'rxjs';
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
import { IncidentRecord } from '../../incidents';
import { MatCardModule } from '@angular/material/card';
import { FmtDatePipe } from '../incidents.component';
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
  incID = inject<string>(MAT_DIALOG_DATA);
  inc$!: Observable<IncidentRecord>;
  form = new FormGroup({
    name: new FormControl(''),
    start: new FormControl(),
    end: new FormControl(),
    description: new FormControl(''),
  });

  constructor(private incSvc: IncidentsService) {}

  ngOnInit() {
    this.inc$ = this.incSvc.getIncident(this.incID).pipe(
      tap((inc) => {
        this.form.patchValue(inc);
      }),
    );
  }

  save() {}

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
  ],
  templateUrl: './incident.component.html',
  styleUrl: './incident.component.scss',
})
export class IncidentComponent {
  incPrime = new Subject<IncidentRecord>();
  inc$!: Observable<IncidentRecord>;
  subscriptions: Subscription = new Subscription();
  dialog = inject(MatDialog);
  incID!: string;

  constructor(
    private route: ActivatedRoute,
    private incSvc: IncidentsService,
  ) {}

  saveIncName(ev: Event) {}

  ngOnInit() {
    this.incID = this.route.snapshot.paramMap.get('id')!;
    this.inc$ = this.incPrime.pipe(map((inc) => inc));
    this.incSvc.getIncident(this.incID).subscribe(this.incPrime);
  }

  editIncident(incID: string) {
    const dialogRef = this.dialog.open(IncidentEditDialogComponent, {
      data: incID,
    });

    dialogRef.afterClosed().subscribe((res) => {
      if (res !== undefined) {
        this.incPrime.next(res);
      }
    });
  }

  ngOnDestroy() {
    this.subscriptions.unsubscribe();
  }
}
