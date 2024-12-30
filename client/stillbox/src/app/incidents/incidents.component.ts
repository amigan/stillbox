import { Component, Pipe, PipeTransform, ViewChild } from '@angular/core';
import { CommonModule, AsyncPipe } from '@angular/common';
import { RouterLink } from '@angular/router';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MatTableModule } from '@angular/material/table';
import {
  MatPaginator,
  MatPaginatorModule,
  PageEvent,
} from '@angular/material/paginator';
import { PrefsService } from '../prefs/prefs.service';
import { MatIconModule } from '@angular/material/icon';
import { SelectionModel } from '@angular/cdk/collections';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { BehaviorSubject, Observable, Subscription } from 'rxjs';
import { map, switchMap } from 'rxjs/operators';
import { IncidentsListParams, IncidentsService } from './incidents.service';
import { IncidentRecord } from '../incidents';

import { TalkgroupService } from '../talkgroups/talkgroups.service';
import { MatFormFieldModule } from '@angular/material/form-field';
import {
  FormControl,
  FormGroup,
  FormsModule,
  ReactiveFormsModule,
} from '@angular/forms';
import { MatInputModule } from '@angular/material/input';
import { MatTimepickerModule } from '@angular/material/timepicker';
import { MatDatepickerModule } from '@angular/material/datepicker';
import { debounceTime } from 'rxjs/operators';
import { ToolbarContextService } from '../navigation/toolbar-context.service';

@Pipe({
  name: 'fmtDate',
  standalone: true,
  pure: true,
})
export class DatePipe implements PipeTransform {
  transform(ts: string, args?: any): string {
    if (!ts) {
      return '\u2014';
    }
    const timestamp = new Date(ts);
    return (
      timestamp.getMonth() +
      1 +
      '/' +
      timestamp.getDate() +
      ' ' +
      timestamp.toLocaleTimeString(navigator.language, {
        hour: '2-digit',
        minute: '2-digit',
        hourCycle: 'h23',
      })
    );
  }
}

const reqPageSize = 200;

@Component({
  selector: 'app-incidents',
  imports: [
    MatIconModule,
    DatePipe,
    MatPaginatorModule,
    MatTableModule,
    AsyncPipe,
    MatFormFieldModule,
    ReactiveFormsModule,
    FormsModule,
    MatInputModule,
    MatDatepickerModule,
    MatTimepickerModule,
    MatCheckboxModule,
    RouterLink,
    CommonModule,
    MatProgressSpinnerModule,
  ],
  templateUrl: './incidents.component.html',
  styleUrl: './incidents.component.scss',
})
export class IncidentsComponent {
  incsResult = new BehaviorSubject(new Array<IncidentRecord>(0));
  @ViewChild('paginator') paginator!: MatPaginator;
  count = 0;
  page = 0;
  perPage = 25;
  pageSizeOptions = [25, 50, 75, 100, 200];
  columns = ['startTime', 'endTime', 'name', 'numCalls', 'edit'];
  curPage = <PageEvent>{ pageIndex: 0, pageSize: 0 };
  currentSet!: IncidentRecord[];
  currentServerPage = 0; // page is never 0, forces load
  isLoading = true;

  selection = new SelectionModel<IncidentRecord>(true, []);

  form = new FormGroup({
    start: new FormControl(null),
    end: new FormControl(null),
    filter: new FormControl(''),
  });

  subscriptions = new Subscription();
  pageWindow = 0;
  fetchIncidents = new BehaviorSubject<IncidentsListParams>(
    this.buildParams(this.curPage, this.curPage.pageIndex),
  );

  constructor(
    private incidentsSvc: IncidentsService,
    private prefsSvc: PrefsService,
    public tcSvc: ToolbarContextService,
    public tgSvc: TalkgroupService,
  ) {
    this.tcSvc.showFilterButton();
  }

  isAllSelected() {
    const numSelected = this.selection.selected.length;
    const numRows = this.curPage.pageSize;
    return numSelected === numRows;
  }

  buildParams(p: PageEvent, serverPage: number): IncidentsListParams {
    const par: IncidentsListParams = {
      start:
        this.form.controls['start'].value != null
          ? new Date(this.form.controls['start'].value!)
          : null,
      page: serverPage,
      perPage: reqPageSize,
      end:
        this.form.controls['end'].value != null
          ? new Date(this.form.controls['end'].value!)
          : null,
      dir: 'desc',
      filter:
        this.form.controls['filter'].value != ''
          ? this.form.controls['filter'].value
          : null,
    };

    return par;
  }

  masterToggle() {
    this.isAllSelected()
      ? this.selection.clear()
      : this.incsResult.value.forEach((row) => this.selection.select(row));
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
      this.prefsSvc.set('incidentsPerPage', p!.pageSize);
    }
    this.getIncidents(p, force);
  }

  refresh() {
    this.selection.clear();
    this.getIncidents(this.curPage, true);
  }

  getIncidents(p: PageEvent, force?: boolean) {
    const pageStart = p.pageIndex * p.pageSize;
    const serverPage = Math.floor(pageStart / reqPageSize) + 1;
    this.pageWindow = pageStart % reqPageSize;
    if (serverPage == this.currentServerPage && !force && this.currentSet) {
      this.incsResult.next(
        this.incsResult
          ? this.currentSet.slice(this.pageWindow, this.pageWindow + p.pageSize)
          : [],
      );
    } else {
      this.currentServerPage = serverPage;
      this.fetchIncidents.next(this.buildParams(p, serverPage));
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
      this.prefsSvc.get('incidentsPerPage').subscribe((cpp) => {
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
      this.fetchIncidents
        .pipe(
          switchMap((params) => {
            return this.incidentsSvc.getIncidents(params);
          }),
        )
        .subscribe((incidents) => {
          this.isLoading = false;
          this.count = incidents.count;
          this.currentSet = incidents.incidents;
          this.incsResult.next(
            this.currentSet
              ? this.currentSet.slice(
                  this.pageWindow,
                  this.pageWindow + this.perPage,
                )
              : [],
          );
        }),
    );
  }

  resetFilter() {
    this.form.reset();
  }
}
