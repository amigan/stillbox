import {
  Component,
  inject,
  Pipe,
  PipeTransform,
  output,
  input,
  Input,
  computed,
  Signal,
  effect,
} from '@angular/core';
import { toObservable } from '@angular/core/rxjs-interop';
import { TalkgroupService, TalkgroupsPaginated } from '../talkgroups.service';
import { NgIconComponent, provideIcons } from '@ng-icons/core';
import {
  ionCreateOutline,
  ionChevronBack,
  ionChevronForward,
} from '@ng-icons/ionicons';
import {
  matFireTruckOutline,
  matLocalPoliceOutline,
  matEmergencyOutline,
  matDirectionsBusOutline,
  matGroupWorkOutline,
} from '@ng-icons/material-icons/outline';
import { Talkgroup, iconMapping } from '../../talkgroup';
import { ActivatedRoute } from '@angular/router';
import { Observable } from 'rxjs';
import { switchMap, tap } from 'rxjs/operators';
import { RouterModule, RouterOutlet, RouterLink } from '@angular/router';
import { CommonModule } from '@angular/common';
import { DomSanitizer, SafeHtml } from '@angular/platform-browser';
import { MatTableDataSource, MatTableModule } from '@angular/material/table';
import {
  MatPaginator,
  MatPaginatorModule,
  PageEvent,
} from '@angular/material/paginator';
import { MatSort } from '@angular/material/sort';
import { PrefsService } from '../../prefs/prefs.service';

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
    NgIconComponent,
    RouterModule,
    RouterLink,
    CommonModule,
    IconifyPipe,
    MatTableModule,
    MatPaginatorModule,
  ],
  templateUrl: './talkgroup-table.component.html',
  styleUrl: './talkgroup-table.component.scss',
  providers: [
    provideIcons({
      ionCreateOutline,
      matFireTruckOutline,
      matLocalPoliceOutline,
      matEmergencyOutline,
      matDirectionsBusOutline,
      matGroupWorkOutline,
      ionChevronBack,
      ionChevronForward,
    }),
  ],
})
export class TalkgroupTableComponent {
  selectedSys: number = 0;
  selectedId: number = 0;
  tgService: TalkgroupService = inject(TalkgroupService);
  prefsService: PrefsService = inject(PrefsService);
  page: number = 0;
  switchPage = output<PageEvent>();
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
    'learned',
    'edit',
  ];

  constructor(private route: ActivatedRoute) {}

  setPage(p: PageEvent) {
    this.switchPage.emit(p);
  }

  ngOnInit() {
    this.talkgroups$.subscribe((event) => {
      if (event != null) {
        this.dataSource.data = event!.talkgroups;
        this.perPage = event!.talkgroups.length;
        this.count = event!.count;
      }
    });
  }
}
