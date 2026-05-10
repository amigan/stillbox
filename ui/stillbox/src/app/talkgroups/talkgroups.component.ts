import { Component, inject, ViewChild } from '@angular/core';
import { TalkgroupService, TalkgroupsPaginated } from './talkgroups.service';
import { ActivatedRoute } from '@angular/router';
import { debounceTime, distinctUntilChanged, switchMap } from 'rxjs/operators';
import { RouterModule, RouterLink } from '@angular/router';

import { TalkgroupTableComponent } from './talkgroup-table/talkgroup-table.component';
import { PageEvent } from '@angular/material/paginator';
import { MatToolbarModule } from '@angular/material/toolbar';
import { PrefsService } from '../prefs/prefs.service';
import { ReplaySubject, Subject, Subscription } from 'rxjs';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MatInputModule } from '@angular/material/input';
import { MatFormFieldModule } from '@angular/material/form-field';
import { FormsModule, ReactiveFormsModule, FormControl } from '@angular/forms';
import { MatIconModule } from '@angular/material/icon';

@Component({
  selector: 'talkgroups',
  imports: [
    RouterModule,
    RouterLink,
    TalkgroupTableComponent,
    MatToolbarModule,
    MatInputModule,
    MatFormFieldModule,
    ReactiveFormsModule,
    FormsModule,
    MatProgressSpinnerModule,
    MatIconModule,
],
  templateUrl: './talkgroups.component.html',
  styleUrl: './talkgroups.component.scss',
  providers: [],
})
export class TalkgroupsComponent {
  @ViewChild('talkgroupTable') talkgroupTable!: TalkgroupTableComponent;
  selectedSys: number = 0;
  selectedId: number = 0;
  tgs!: TalkgroupsPaginated;
  tgService: TalkgroupService = inject(TalkgroupService);
  prefsService: PrefsService = inject(PrefsService);
  perPage = 0;
  filter = new FormControl('');
  curPage = <PageEvent>{ pageIndex: 0, pageSize: this.perPage };
  private getPage = new ReplaySubject<PageEvent>(1);
  constructor(private route: ActivatedRoute) {}
  subs = new Subscription();

  pageReset: Subject<void> = new Subject<void>();

  resetPage() {
    this.pageReset.next();
  }

  switchPage(p: PageEvent) {
    this.curPage = p;

    if (p.pageSize != this.perPage) {
      this.perPage = p.pageSize;
      this.prefsService.set('tgsPerPage', this.perPage);
    }
    this.getPage.next(p);
  }

  ngOnDestroy() {
    this.subs.unsubscribe();
  }

  ngOnInit() {
    this.prefsService.get('tgsPerPage').subscribe((tgpp) => {
      if (!tgpp) {
        tgpp = 25;
      }
      if (tgpp != this.perPage) {
        this.perPage = tgpp;

        this.switchPage(<PageEvent>{
          pageIndex: 0,
          pageSize: tgpp,
        });
      }
    });
    this.subs.add(
      this.getPage
        .pipe(
          switchMap((p) =>
            this.route.paramMap.pipe(
              switchMap((params) => {
                this.selectedSys = Number(params.get('sys'));
                this.selectedId = Number(params.get('tg'));
                return this.tgService.getTalkgroupsPag({
                  page: p.pageIndex + 1,
                  perPage: p.pageSize,
                  filter: this.filter.value == '' ? null : this.filter.value,
                });
              }),
            ),
          ),
        )
        .subscribe((ev) => {
          this.tgs = ev;
        }),
    );
  }

  zeroPage(): PageEvent {
    return <PageEvent>{
      pageIndex: 0,
      pageSize: this.curPage.pageSize,
    };
  }

  ngAfterViewInit() {
    this.filter.valueChanges
      .pipe(debounceTime(500))
      .pipe(distinctUntilChanged())
      .subscribe(() => {
        this.filterChange();
      });
  }

  changeFilter(f: string) {
    this.filter.setValue(f);
  }
  filterChange() {
    this.curPage.pageIndex = 0;
    this.resetPage();
    this.switchPage(this.curPage);
  }
}
