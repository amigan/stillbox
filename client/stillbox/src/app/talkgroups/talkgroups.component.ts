import { Component, inject, Pipe, PipeTransform } from '@angular/core';
import { TalkgroupService, TalkgroupsPaginated } from './talkgroups.service';
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
import { Talkgroup, iconMapping } from '../talkgroup';
import { ActivatedRoute } from '@angular/router';
import { Observable } from 'rxjs';
import { switchMap, tap } from 'rxjs/operators';
import { RouterModule, RouterOutlet, RouterLink } from '@angular/router';
import { CommonModule } from '@angular/common';
import { DomSanitizer, SafeHtml } from '@angular/platform-browser';
import { TalkgroupTableComponent } from './talkgroup-table/talkgroup-table.component';
import { PageEvent } from '@angular/material/paginator';
import { MatToolbarModule } from '@angular/material/toolbar';
import { PrefsService } from '../prefs/prefs.service';

@Component({
  selector: 'talkgroups',
  standalone: true,
  imports: [
    RouterModule,
    RouterLink,
    CommonModule,
    TalkgroupTableComponent,
    MatToolbarModule,
  ],
  templateUrl: './talkgroups.component.html',
  styleUrl: './talkgroups.component.scss',
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
export class TalkgroupsComponent {
  selectedSys: number = 0;
  selectedId: number = 0;
  tgs!: TalkgroupsPaginated;
  tgService: TalkgroupService = inject(TalkgroupService);
  prefsService: PrefsService = inject(PrefsService);
  perPage = 25;

  constructor(private route: ActivatedRoute) {}

  switchPage(p: PageEvent | undefined) {
    this.route.paramMap
      .pipe(
        switchMap((params) => {
          this.perPage = p!.pageSize;
          this.selectedSys = Number(params.get('sys'));
          this.selectedId = Number(params.get('tg'));
          return this.tgService.getTalkgroupsPag({
            page: p!.pageIndex + 1,
            perPage: p!.pageSize,
          });
        }),
      )
      .subscribe((ev) => {
        this.tgs = ev;
      });
    this.prefsService.setTGsPerPage(p!.pageSize);
  }

  ngOnInit() {
    this.prefsService.prefs$.subscribe((event) => {
      this.perPage = event?.tgsPerPage ?? 25;
      this.switchPage(<PageEvent>{ pageIndex: 0, pageSize: this.perPage });
    });
  }
}
