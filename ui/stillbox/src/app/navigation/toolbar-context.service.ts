import { Injectable, inject } from '@angular/core';
import { BehaviorSubject, Observable, Subscription } from 'rxjs';
import { BreakpointObserver, Breakpoints } from '@angular/cdk/layout';
import { map, shareReplay } from 'rxjs/operators';

@Injectable({
  providedIn: 'root',
})
export class ToolbarContextService {
  private breakpointObserver = inject(BreakpointObserver);
  filterBtn: BehaviorSubject<boolean>;
  isHandset$: Observable<boolean>;
  filterPanel: BehaviorSubject<boolean> = new BehaviorSubject(false);
  subscriptions = new Subscription();

  showFilterButton() {
    this.filterBtn.next(true);
  }

  hideFilterButton() {
    this.filterBtn.next(false);
  }

  showFilterPanel() {
    this.filterPanel.next(true);
  }

  hideFilterPanel() {
    this.filterPanel.next(false);
  }

  ngOnDestroy() {
    this.subscriptions.unsubscribe();
  }

  constructor() {
    this.filterBtn = new BehaviorSubject(false);
    this.isHandset$ = this.breakpointObserver.observe(Breakpoints.Handset).pipe(
      map((result) => result.matches),
      shareReplay(),
    );
    this.subscriptions.add(this.isHandset$.subscribe(this.filterPanel));
  }
}
