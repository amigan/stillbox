import { Component, inject, Input, ViewChild } from '@angular/core';
import { BreakpointObserver, Breakpoints } from '@angular/cdk/layout';
import { AsyncPipe } from '@angular/common';
import { MatToolbarModule } from '@angular/material/toolbar';
import { MatButtonModule } from '@angular/material/button';
import { MatDrawer, MatSidenavModule } from '@angular/material/sidenav';
import { MatListModule } from '@angular/material/list';
import { MatIconModule } from '@angular/material/icon';
import { Observable } from 'rxjs';
import { map, shareReplay } from 'rxjs/operators';
import { FormsModule } from '@angular/forms';
import {
  RouterModule,
  RouterOutlet,
  RouterLink,
  RouterLinkActive,
} from '@angular/router';
import { CommonModule } from '@angular/common';

import { Subscription } from 'rxjs';

interface HomeRoute {
  name: string;
  url: string;
  icon: string;
  exact: boolean;
}

@Component({
  selector: 'app-navigation',
  templateUrl: './navigation.component.html',
  styleUrl: './navigation.component.scss',
  standalone: true,
  imports: [
    MatToolbarModule,
    MatButtonModule,
    MatSidenavModule,
    MatListModule,
    MatIconModule,
    AsyncPipe,
    CommonModule,
    RouterOutlet,
    RouterModule,
    RouterLink,
    RouterLinkActive,
    FormsModule,
  ],
  providers: [],
})
export class NavigationComponent {
  private breakpointObserver = inject(BreakpointObserver);
  @ViewChild('drawer', { static: true }) drawer!: MatDrawer;

  toggleSubscription!: Subscription;
  isExpanded = false;

  @Input() events!: Observable<void>;

  ngOnInit() {
    this.toggleSubscription = this.events.subscribe(() => {
      this.isExpanded = !this.isExpanded;
    });
  }

  ngOnDestroy() {
    this.toggleSubscription.unsubscribe();
  }

  homeRoutes = <HomeRoute[]>[
    {
      name: 'Home',
      url: '/',
      icon: 'home',
      exact: true,
    },
    {
      name: 'Talkgroups',
      url: '/talkgroups',
      icon: 'forum',
    },
    {
      name: 'Calls',
      url: '/calls',
      icon: 'campaign',
    },
    {
      name: 'Incidents',
      url: '/incidents',
      icon: 'newspaper',
    },
    {
      name: 'Alerts',
      url: '/alerts',
      icon: 'notifications',
    },
  ];

  isHandset$: Observable<boolean> = this.breakpointObserver
    .observe(Breakpoints.Handset)
    .pipe(
      map((result) => result.matches),
      shareReplay(),
    );
}
