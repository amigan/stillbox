import { Component, inject, OnDestroy, OnInit } from '@angular/core';
import {
  MAT_DIALOG_DATA,
  MatDialogModule,
  MatDialogRef,
} from '@angular/material/dialog';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatListModule } from '@angular/material/list';
import {
  IncidentsListParams,
  IncidentsPaginated,
  IncidentsService,
} from '../incidents.service';
import {
  BehaviorSubject,
  debounceTime,
  distinctUntilChanged,
  finalize,
  Subscription,
  switchMap,
  tap,
} from 'rxjs';
import { IncidentRecord } from '../../incidents';
import { FormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { FmtDatePipe } from '../incidents.component';

export interface SelectIncidentDialogData {
  preselectedIncidentId?: string | null;
}

const PAGE_SIZE = 25;

@Component({
  selector: 'app-select-incident-dialog',
  imports: [
    MatListModule,
    MatFormFieldModule,
    MatInputModule,
    MatDialogModule,
    MatButtonModule,
    FormsModule,
    MatProgressSpinnerModule,
    FmtDatePipe,
  ],
  templateUrl: './select-incident-dialog.component.html',
  styleUrl: './select-incident-dialog.component.scss',
})
export class SelectIncidentDialogComponent implements OnInit, OnDestroy {
  dialogRef = inject(MatDialogRef<SelectIncidentDialogComponent>);
  private data = inject<SelectIncidentDialogData>(MAT_DIALOG_DATA, {
    optional: true,
  });

  private incSvc = inject(IncidentsService);
  private findStr = new BehaviorSubject<string>('');
  private sub = new Subscription();

  sel: string | null = null;
  accumulated: IncidentRecord[] = [];
  totalCount = 0;
  currentPage = 1;
  private currentFilter = '';
  loading = false;
  initialLoad = true;

  ngOnInit() {
    if (this.data?.preselectedIncidentId) {
      this.sel = this.data.preselectedIncidentId;
    }

    this.sub.add(
      this.findStr
        .pipe(
          debounceTime(300),
          distinctUntilChanged(),
          tap(() => {
            this.currentPage = 1;
            this.accumulated = [];
            this.initialLoad = true;
          }),
          switchMap((filter) => {
            this.currentFilter = filter;
            this.loading = true;
            return this.incSvc
              .getIncidents(this.buildParams(filter, 1))
              .pipe(finalize(() => (this.loading = false)));
          }),
        )
        .subscribe((pag: IncidentsPaginated) => {
          this.accumulated = pag.incidents;
          this.totalCount = pag.count;
          this.currentPage = 1;
          this.initialLoad = false;
        }),
    );
  }

  ngOnDestroy() {
    this.sub.unsubscribe();
  }

  private buildParams(
    filter: string,
    page: number,
  ): IncidentsListParams {
    const trimmed = filter.trim();
    return {
      start: null,
      end: null,
      filter: trimmed !== '' ? trimmed : null,
      dir: 'desc',
      orderBy: 'start',
      page,
      perPage: PAGE_SIZE,
    };
  }

  search(term: string) {
    this.findStr.next(term);
  }

  loadMore() {
    if (
      this.loading ||
      this.accumulated.length >= this.totalCount ||
      this.initialLoad
    ) {
      return;
    }
    const next = this.currentPage + 1;
    this.loading = true;
    this.sub.add(
      this.incSvc
        .getIncidents(this.buildParams(this.currentFilter, next))
        .pipe(finalize(() => (this.loading = false)))
        .subscribe((pag: IncidentsPaginated) => {
          this.accumulated = [...this.accumulated, ...pag.incidents];
          this.currentPage = next;
        }),
    );
  }

  cancel() {
    this.dialogRef.close(null);
  }

  pickAndClose(id: string) {
    this.sel = id;
    this.save();
  }

  save() {
    if (!this.sel) {
      return;
    }
    this.dialogRef.close(this.sel);
  }

  canLoadMore(): boolean {
    return (
      !this.loading &&
      !this.initialLoad &&
      this.accumulated.length < this.totalCount
    );
  }
}
