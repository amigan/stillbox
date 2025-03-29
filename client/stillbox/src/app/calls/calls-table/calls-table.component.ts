import {
  Component,
  computed,
  ElementRef,
  EventEmitter,
  inject,
  input,
  Output,
  Sanitizer,
  SecurityContext,
  signal,
  Signal,
  ViewChild,
} from '@angular/core';
import { CommonModule, AsyncPipe } from '@angular/common';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { MatTableDataSource, MatTableModule } from '@angular/material/table';
import {
  MatPaginator,
  MatPaginatorModule,
  PageEvent,
} from '@angular/material/paginator';
import { MatIconModule } from '@angular/material/icon';
import { SelectionModel } from '@angular/cdk/collections';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { BehaviorSubject, Observable, Subject, Subscription } from 'rxjs';
import { map, switchMap } from 'rxjs/operators';
import {
  CallsListParams,
  CallsService,
  DatePipe,
  FixedPointPipe,
  TalkerPipe,
  TalkgroupPipe,
  TimePipe,
  TranscriptPipe,
} from './../calls.service';
import { CallRecord } from '../../calls';

import { MatFormFieldModule } from '@angular/material/form-field';
import {
  FormControl,
  FormGroup,
  FormsModule,
  ReactiveFormsModule,
} from '@angular/forms';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { CallPlayerComponent } from '../player/call-player/call-player.component';
import { MatMenuModule } from '@angular/material/menu';
import { MatTooltipModule } from '@angular/material/tooltip';
import { PlayerService } from '../player/player.service';
import { DomSanitizer } from '@angular/platform-browser';
import { toSignal } from '@angular/core/rxjs-interop';
import { IncidentCall } from '../../incidents';

export const PER_PAGE_DEFAULT = 25;

@Component({
  selector: 'calls-table',
  imports: [
    MatIconModule,
    FixedPointPipe,
    TalkgroupPipe,
    TalkerPipe,
    TranscriptPipe,
    TimePipe,
    DatePipe,
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
    CallPlayerComponent,
    MatMenuModule,
    MatTooltipModule,
  ],
  templateUrl: './calls-table.component.html',
  styleUrl: './calls-table.component.scss',
})
export class CallsTableComponent {
  @ViewChild('paginator') paginator!: MatPaginator;
  callsResult = input<CallRecord[] | MatTableDataSource<IncidentCall>| null>();
  @Output() searchTGFilter: EventEmitter<string | null> = new EventEmitter();
  @Output() searchSrcFilter: EventEmitter<string | null> = new EventEmitter();
  @Output() setPage: EventEmitter<PageEvent> = new EventEmitter();
  curPage = input<PageEvent>();
  showTranscripts = input<boolean | null>();
  searched = input<boolean>();
  @ViewChild('callsTable', { read: ElementRef }) callsTable!: ElementRef;
  page = 0;
  count = 0;
  pageSizeOptions = [25, 50, 75, 100, 200];
  columns = [
    'select',
    'play',
    'date',
    'time',
    'system',
    'group',
    'talkgroup',
    'talker',
    'transcript',
    'duration',
  ];
  curLen = 0;
  tableFG = new FormGroup({
    downloadMode: new FormControl<boolean>(false),
  });
  downloadMode = toSignal(
    this.tableFG.controls.downloadMode.valueChanges.pipe(
      map((v) => {
        if (v == true) {
          return true;
        }

        return false;
      }),
    ),
  );

  selection = new SelectionModel<CallRecord>(true, []);
  getColumns = computed(() => {
    if (this.showTranscripts()) {
      return this.columns;
    }
    return this.columns.filter((tx) => tx != 'transcript');
  });

  constructor(private sanitizer: DomSanitizer) {}

  isAllSelected() {
    const numSelected = this.selection.selected.length;
    const numRows = this.curLen;
    return numSelected === numRows;
  }

  masterToggle() {
    let cr: CallRecord[] | MatTableDataSource<IncidentCall> | null | undefined = this.callsResult();
    if (this.isAllSelected() || cr === undefined || cr === null) {
      this.selection.clear();
      return;
    }
    if (cr instanceof MatTableDataSource) {
      cr.data.forEach((row) => this.selection.select(row));
      return;
    }
    cr.forEach((row) => this.selection.select(row));
  }

  sanitize(s: string): string | null {
    return this.sanitizer.sanitize(SecurityContext.HTML, s);
  }
}
