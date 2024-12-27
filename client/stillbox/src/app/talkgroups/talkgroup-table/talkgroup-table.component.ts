import {
  Component,
  inject,
  Pipe,
  Input,
  PipeTransform,
  output,
  ViewChild,
  input,
} from '@angular/core';
import { toObservable } from '@angular/core/rxjs-interop';
import { TalkgroupService, TalkgroupsPaginated } from '../talkgroups.service';
import { Talkgroup, iconMapping } from '../../talkgroup';
import { ActivatedRoute } from '@angular/router';
import { RouterModule, RouterLink } from '@angular/router';
import { CommonModule } from '@angular/common';
import { DomSanitizer, SafeHtml } from '@angular/platform-browser';
import { MatTableDataSource, MatTableModule } from '@angular/material/table';
import {
  MatPaginator,
  MatPaginatorModule,
  PageEvent,
} from '@angular/material/paginator';
import { PrefsService } from '../../prefs/prefs.service';
import { MatIconModule } from '@angular/material/icon';
import { MatChipsModule } from '@angular/material/chips';
import { SelectionModel } from '@angular/cdk/collections';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { Observable, Subscription } from 'rxjs';

@Pipe({
  standalone: true,
  name: 'iconify',
})
export class IconifyPipe implements PipeTransform {
  transform(value: Talkgroup): Talkgroup {
    if (value?.metadata?.icon != null) {
      value.iconSvg = iconMapping[value?.metadata?.icon];
    } else if (value?.metadata?.icon == undefined) {
      value.iconSvg = iconMapping[''];
    }
    return value;
  }
}

@Pipe({
  standalone: true,
  name: 'sanitizeHtml',
})
export class SanitizeHtmlPipe implements PipeTransform {
  constructor(private _sanitizer: DomSanitizer) {}

  transform(v: string): SafeHtml {
    return this._sanitizer.bypassSecurityTrustHtml(v);
  }
}

@Component({
  selector: 'talkgroup-table',
  imports: [
    RouterModule,
    RouterLink,
    MatIconModule,
    CommonModule,
    IconifyPipe,
    MatTableModule,
    MatPaginatorModule,
    MatCheckboxModule,
    MatChipsModule,
  ],
  templateUrl: './talkgroup-table.component.html',
  styleUrl: './talkgroup-table.component.scss',
  providers: [],
})
export class TalkgroupTableComponent {
  selectedSys: number = 0;
  selectedId: number = 0;
  tgService: TalkgroupService = inject(TalkgroupService);
  prefsService: PrefsService = inject(PrefsService);
  page: number = 0;
  switchPage = output<PageEvent>();
  changeFilter = output<string>();
  talkgroups = input<TalkgroupsPaginated>();
  talkgroups$ = toObservable(this.talkgroups);
  dataSource = new MatTableDataSource<Talkgroup>();
  pageSizeOptions = [25, 50, 75, 100, 200];
  perPage: number = 25;
  count = 0;
  columns = [
    'select',
    'icon',
    'sysID',
    'sysName',
    'group',
    'name',
    'alphaTag',
    'tgid',
    'tags',
    'learned',
    'edit',
  ];
  selection = new SelectionModel<Talkgroup>(true, []);
  private resetPageSub!: Subscription;
  @Input() resetPage!: Observable<void>;
  @ViewChild('paginator') paginator!: MatPaginator;
  suppress = false;

  constructor(private route: ActivatedRoute) {}

  setPage(p: PageEvent) {
    // don't needlessly request page 0 if we were asked merely to reset the state of the control
    this.selection.clear();
    if (!this.suppress) {
      this.switchPage.emit(p);
    }
  }

  ngOnInit() {
    this.resetPageSub = this.resetPage.subscribe(() => {
      this.suppress = true;
      this.paginator.firstPage();
      this.selection.clear();
      this.suppress = false;
    });

    this.prefsService.get('tgsPerPage').subscribe((tgpp) => {
      this.perPage = tgpp;
    });
    this.talkgroups$.subscribe((event) => {
      if (event != null) {
        this.dataSource.data = event!.talkgroups;
        this.count = event!.count;
      }
    });
  }

  ngOnDestroy() {
    this.resetPageSub.unsubscribe();
  }

  searchChip(event: MouseEvent) {
    // not a fan of how this looks, but it works...
    this.changeFilter.emit(
      (event.target as Element).childNodes[0].textContent ?? '',
    );
  }

  isAllSelected() {
    const numSelected = this.selection.selected.length;
    const numRows = this.dataSource.data.length;
    return numSelected === numRows;
  }

  masterToggle() {
    this.isAllSelected()
      ? this.selection.clear()
      : this.dataSource.data.forEach((row) => this.selection.select(row));
  }
}
