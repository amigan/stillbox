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
import { BehaviorSubject, Subscription } from 'rxjs';
import { switchMap } from 'rxjs/operators';
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
import { debounceTime } from 'rxjs/operators';
import { ToolbarContextService } from '../navigation/toolbar-context.service';
import { AvatarComponent } from '../user/avatar/avatar.component';

@Pipe({
  name: 'fmtDate',
  standalone: true,
  pure: true,
})
export class FmtDatePipe implements PipeTransform {
  transform(ts: string | Date | null | undefined, args?: any): string {
    if (!ts) {
      return '\u2014';
    }
    let timestamp: Date;
    if (ts instanceof Date) {
      timestamp = ts;
    } else {
      timestamp = new Date(ts);
    }
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

@Component({
  selector: 'app-incidents',
  imports: [
    MatIconModule,
    FmtDatePipe,
    MatPaginatorModule,
    MatTableModule,
    AsyncPipe,
    MatFormFieldModule,
    ReactiveFormsModule,
    FormsModule,
    MatInputModule,
    MatCheckboxModule,
    RouterLink,
    CommonModule,
    MatProgressSpinnerModule,
    AvatarComponent,
  ],
  templateUrl: './incidents.component.html',
  styleUrl: './incidents.component.scss',
})
export class IncidentsComponent {
  incsResult = new BehaviorSubject(new Array<IncidentRecord>(0));
  @ViewChild('paginator') paginator!: MatPaginator;
  count = 0;
  curLen = 0;
  page = 0;
  perPage = 25;
  pageSizeOptions = [25, 50, 75, 100, 200];
  columns = [
    'select',
    'owner',
    'startTime',
    'endTime',
    'name',
    'numCalls',
    'edit',
  ];
  curPage = <PageEvent>{ pageIndex: 0, pageSize: 0 };
  currentSet!: IncidentRecord[];
  isLoading = true;

  selection = new SelectionModel<IncidentRecord>(true, []);

  form = new FormGroup({
    start: new FormControl(null),
    end: new FormControl(null),
    filter: new FormControl(''),
  });

  subscriptions = new Subscription();
  fetchIncidents = new BehaviorSubject<IncidentsListParams>(
    this.buildParams(this.curPage),
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
    const numRows = this.curLen;
    return numSelected === numRows;
  }

  buildParams(p: PageEvent): IncidentsListParams {
    const par: IncidentsListParams = {
      start:
        this.form.controls['start'].value != null
          ? new Date(this.form.controls['start'].value!)
          : null,
      page: p.pageIndex + 1,
      perPage: p.pageSize,
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
    this.fetchIncidents.next(this.buildParams(p));
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
          this.incsResult.next(this.currentSet);
        }),
    );
    this.subscriptions.add(
      this.incsResult.subscribe((cr) => {
        this.curLen = cr.length;
      }),
    );
  }

  resetFilter() {
    this.form.reset();
  }
}
