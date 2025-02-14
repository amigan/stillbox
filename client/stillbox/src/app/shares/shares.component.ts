import { Component, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MatTableModule } from '@angular/material/table';
import {
  MatPaginator,
  MatPaginatorModule,
  PageEvent,
} from '@angular/material/paginator';
import { MatIconModule } from '@angular/material/icon';
import { SelectionModel } from '@angular/cdk/collections';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { BehaviorSubject, Subscription } from 'rxjs';
import { switchMap } from 'rxjs/operators';

import { MatFormFieldModule } from '@angular/material/form-field';
import {
  FormsModule,
  ReactiveFormsModule,
} from '@angular/forms';
import { MatInputModule } from '@angular/material/input';
import {
  ShareListParams,
  ShareRecord,
  ShareService,
} from '../share/share.service';
import { FmtDatePipe } from '../incidents/incidents.component';

const reqPageSize = 200;

@Component({
  selector: 'app-shares',
  imports: [
    MatIconModule,
    MatPaginatorModule,
    MatTableModule,
    MatFormFieldModule,
    ReactiveFormsModule,
    FormsModule,
    FmtDatePipe,
    MatInputModule,
    MatCheckboxModule,
    CommonModule,
    MatProgressSpinnerModule,
  ],
  templateUrl: './shares.component.html',
  styleUrl: './shares.component.scss',
})
export class SharesComponent {
  sharesResult = new BehaviorSubject(new Array<ShareRecord>(0));
  selection = new SelectionModel<ShareRecord>(true, []);

  @ViewChild('paginator') paginator!: MatPaginator;
  count = 0;
  curLen = 0;
  page = 0;
  perPage = 25;
  pageSizeOptions = [25, 50, 75, 100, 200];
  columns = ['select', 'type', 'link', 'date', 'owner', 'delete'];
  curPage = <PageEvent>{ pageIndex: 0, pageSize: 0 };
  currentSet!: ShareRecord[];
  currentServerPage = 0; // page is never 0, forces load
  isLoading = true;
  subscriptions = new Subscription();
  pageWindow = 0;
  fetchIncidents = new BehaviorSubject<ShareListParams>(
    this.buildParams(this.curPage, this.curPage.pageIndex),
  );

  constructor(private sharesSvc: ShareService) {}

  isAllSelected() {
    const numSelected = this.selection.selected.length;
    const numRows = this.curLen;
    return numSelected === numRows;
  }

  buildParams(p: PageEvent, serverPage: number): ShareListParams {
    const par: ShareListParams = {
      page: serverPage,
      perPage: reqPageSize,
      dir: 'asc',
    };

    return par;
  }

  masterToggle() {
    this.isAllSelected()
      ? this.selection.clear()
      : this.sharesResult.value.forEach((row) => this.selection.select(row));
  }

  setPage(p: PageEvent, force?: boolean) {
    this.selection.clear();
    this.curPage = p;
    if (p && p!.pageSize != this.perPage) {
      this.perPage = p!.pageSize;
    }
    this.getShares(p, force);
  }

  refresh() {
    this.selection.clear();
    this.getShares(this.curPage, true);
  }

  getShares(p: PageEvent, force?: boolean) {
    const pageStart = p.pageIndex * p.pageSize;
    const serverPage = Math.floor(pageStart / reqPageSize) + 1;
    this.pageWindow = pageStart % reqPageSize;
    if (serverPage == this.currentServerPage && !force && this.currentSet) {
      this.sharesResult.next(
        this.sharesResult
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
    this.subscriptions.unsubscribe();
  }

  ngOnInit() {
    let cpp = 25;
    this.perPage = cpp;

    this.setPage(<PageEvent>{
      pageIndex: 0,
      pageSize: cpp,
    });
    this.subscriptions.add(
      this.fetchIncidents
        .pipe(
          switchMap((params) => {
            return this.sharesSvc.getShares(params);
          }),
        )
        .subscribe((shares) => {
          this.isLoading = false;
          this.count = shares.totalCount;
          this.currentSet = shares.shares;
          this.sharesResult.next(
            this.currentSet
              ? this.currentSet.slice(
                  this.pageWindow,
                  this.pageWindow + this.perPage,
                )
              : [],
          );
        }),
    );
    this.subscriptions.add(
      this.sharesResult.subscribe((cr) => {
        this.curLen = cr.length;
      }),
    );
  }

  deleteShare(shareID: string) {
    if (confirm('Are you sure you want to delete this share?')) {
      this.sharesSvc.deleteShare(shareID).subscribe({
        next: () => {
          this.fetchIncidents.next(
            this.buildParams(this.curPage, this.curPage.pageIndex),
          );
        },
        error: (err) => {
          alert(err);
        },
      });
    }
  }
}
