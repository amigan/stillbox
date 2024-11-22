import { Component, inject } from '@angular/core';
import { TalkgroupService } from './talkgroups.service';
import { Talkgroup } from '../talkgroup';
import { NgIconComponent, provideIcons } from '@ng-icons/core';
import { ionCreateOutline } from '@ng-icons/ionicons';
import { ActivatedRoute } from '@angular/router';
import { Observable } from 'rxjs';
import { switchMap } from 'rxjs/operators';
import { RouterModule, RouterOutlet, RouterLink } from '@angular/router';
import { CommonModule } from '@angular/common';

@Component({
  selector: 'talkgroups',
  standalone: true,
  imports: [
    NgIconComponent,
    RouterOutlet,
    RouterModule,
    RouterLink,
    CommonModule,
  ],
  templateUrl: './talkgroups.component.html',
  styleUrl: './talkgroups.component.css',
  providers: [provideIcons({ ionCreateOutline })],
})
export class TalkgroupsComponent {
  selectedSys: number = 0;
  selectedId: number = 0;
  talkgroups$!: Observable<Talkgroup[]>;
  tgService: TalkgroupService = inject(TalkgroupService);

  constructor(private route: ActivatedRoute) {}

  ngOnInit() {
    this.talkgroups$ = this.route.paramMap.pipe(
      switchMap((params) => {
        this.selectedSys = Number(params.get('sys'));
        this.selectedId = Number(params.get('tg'));
        return this.tgService.getTalkgroups();
      }),
    );
  }
}
