import { Component, inject, Pipe, PipeTransform } from '@angular/core';
import { TalkgroupService, TalkgroupsPaginated } from './talkgroups.service';
import { NgIconComponent, provideIcons } from '@ng-icons/core';
import { ionCreateOutline, ionChevronBack, ionChevronForward  } from '@ng-icons/ionicons';
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
  name: 'sanitizeHtml'
})
export class SanitizeHtmlPipe implements PipeTransform {

  constructor(private _sanitizer:DomSanitizer) {
  }

  transform(v:string):SafeHtml {
    return this._sanitizer.bypassSecurityTrustHtml(v);
  }
}


@Component({
  selector: 'talkgroups',
  standalone: true,
  imports: [
    NgIconComponent,
    RouterOutlet,
    RouterModule,
    RouterLink,
    CommonModule,
    IconifyPipe,
    SanitizeHtmlPipe,
  ],
  templateUrl: './talkgroups.component.html',
  styleUrl: './talkgroups.component.css',
  providers: [provideIcons({ ionCreateOutline,
     matFireTruckOutline,
  matLocalPoliceOutline,
  matEmergencyOutline,
  matDirectionsBusOutline,
  matGroupWorkOutline,
  ionChevronBack,
  ionChevronForward,
})],
})
export class TalkgroupsComponent {
  selectedSys: number = 0;
  selectedId: number = 0;
  talkgroups$!: Observable<TalkgroupsPaginated>;
  tgService: TalkgroupService = inject(TalkgroupService);
  page: number = 1;
  perPage: number = 20;
  totalPages: number = 1;

  constructor(private route: ActivatedRoute) {}

  prevPage() {
    if (this.page > 1) {
      this.page--;
      this.fetchTGs();
    }
  }

  nextPage() {
    if (this.page < this.totalPages) {
      this.page++;
      this.fetchTGs();
    }
  }

  fetchTGs() {
    this.talkgroups$ = this.route.paramMap.pipe(
      switchMap((params) => {
        this.selectedSys = Number(params.get('sys'));
        this.selectedId = Number(params.get('tg'));
        return this.tgService.getTalkgroupsPag({page: this.page, perPage: this.perPage}).pipe(
          tap((event) => {
            this.totalPages = Math.ceil(event.count/this.perPage);
          })
        );
      }),
    );
  }

  ngOnInit() {
    this.fetchTGs();
  }
}
