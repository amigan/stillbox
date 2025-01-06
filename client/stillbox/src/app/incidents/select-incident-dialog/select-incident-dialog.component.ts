import { AsyncPipe } from '@angular/common';
import { Component, inject, ViewChild } from '@angular/core';
import { MatDialogModule, MatDialogRef } from '@angular/material/dialog';
import { MatInputModule } from '@angular/material/input';
import { MatListModule, MatSelectionList } from '@angular/material/list';
import {
  IncidentsListParams,
  IncidentsPaginated,
  IncidentsService,
} from '../incidents.service';
import {
  BehaviorSubject,
  combineLatest,
  debounceTime,
  distinctUntilChanged,
  map,
  merge,
  Observable,
  switchMap,
} from 'rxjs';
import { IncidentRecord } from '../../incidents';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';

@Component({
  selector: 'app-select-incident-dialog',
  imports: [
    MatListModule,
    MatInputModule,
    AsyncPipe,
    MatDialogModule,
    MatButtonModule,
    FormsModule,
  ],
  templateUrl: './select-incident-dialog.component.html',
  styleUrl: './select-incident-dialog.component.scss',
})
export class SelectIncidentDialogComponent {
  dialogRef = inject(MatDialogRef<SelectIncidentDialogComponent>);
  allIncs$!: Observable<IncidentRecord[]>;
  incs$!: Observable<IncidentRecord[]>;
  findStr = new BehaviorSubject<string>('');
  sel!: string;
  constructor(private incSvc: IncidentsService) {}

  ngOnInit() {
    this.incs$ = this.findStr.pipe(
      debounceTime(300),
      distinctUntilChanged(),
      switchMap((sub) =>
        this.incSvc.getIncidents(<IncidentsListParams>{ filter: sub }),
      ),
      map((incidents) => incidents.incidents),
    );
  }

  search(term: string) {
    this.findStr.next(term);
  }

  cancel() {
    this.dialogRef.close(null);
  }

  save() {
    this.dialogRef.close(this.sel);
  }
}
