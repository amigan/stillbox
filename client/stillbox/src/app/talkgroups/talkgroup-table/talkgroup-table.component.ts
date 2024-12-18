import {
  Component,
  inject,
  Pipe,
  PipeTransform,
  output,
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
import { MatPaginatorModule, PageEvent } from '@angular/material/paginator';
import { PrefsService } from '../../prefs/prefs.service';
import { MatIconModule } from '@angular/material/icon';
import {
  MatChipSelectionChange,
  MatChipsModule,
} from '@angular/material/chips';

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
  standalone: true,
  imports: [
    RouterModule,
    RouterLink,
    MatIconModule,
    CommonModule,
    IconifyPipe,
    MatTableModule,
    MatPaginatorModule,
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
  pageSizeOptions = [25, 50, 75, 100, 200, 500];
  perPage: number = 25;
  count = 0;
  columns = [
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

  constructor(private route: ActivatedRoute) {}

  setPage(p: PageEvent) {
    this.switchPage.emit(p);
  }

  ngOnInit() {
    this.perPage = this.prefsService.last.tgsPerPage;
    this.talkgroups$.subscribe((event) => {
      if (event != null) {
        this.dataSource.data = event!.talkgroups;
        this.count = event!.count;
      }
    });
  }

  searchChip(event: MouseEvent) {
    // not a fan of how this looks, but it works...
    this.changeFilter.emit(
      (event.target as Element).childNodes[0].textContent ?? '',
    );
  }
}
