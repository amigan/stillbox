import { Component, Input, ViewChild } from '@angular/core';
import { AsyncPipe } from '@angular/common';
import { MatToolbarModule } from '@angular/material/toolbar';
import { MatButtonModule } from '@angular/material/button';
import { MatDrawer, MatSidenavModule } from '@angular/material/sidenav';
import { MatListModule } from '@angular/material/list';
import { MatIconModule } from '@angular/material/icon';
import { Observable } from 'rxjs';
import { FormsModule } from '@angular/forms';
import {
  RouterModule,
  RouterOutlet,
  RouterLink,
  RouterLinkActive,
} from '@angular/router';
import { CommonModule } from '@angular/common';

import { Subscription } from 'rxjs';
import { ToolbarContextService } from './toolbar-context.service';

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
  @ViewChild('drawer', { static: true }) drawer!: MatDrawer;

  toggleSubscription!: Subscription;
  isExpanded = false;
  showFilter = false;
  showFilterSub!: Subscription;

  @Input() events!: Observable<void>;

  constructor(public tcSvc: ToolbarContextService) {}

  ngOnInit() {
    this.toggleSubscription = this.events.subscribe(() => {
      this.isExpanded = !this.isExpanded;
    });
  }

  ngAfterViewChecked() {
    this.showFilterSub = this.tcSvc.filterBtn.subscribe((show) => {
      this.showFilter = show;
    });
  }

  ngOnDestroy() {
    this.toggleSubscription.unsubscribe();
    this.showFilterSub.unsubscribe();
  }

  homeRoutes = <HomeRoute[]>[
    {
      name: 'Home',
      url: '/',
      icon: 'home',
      exact: true,
    },
    {
      name: 'Calls',
      url: '/calls',
      icon: 'campaign',
    },
    {
      name: 'Talkgroups',
      url: '/talkgroups',
      icon: 'forum',
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
    {
      name: 'Shares',
      url: '/shares',
      icon: 'share',
    },
  ];

  toggleFilterPanel() {
    this.tcSvc.filterPanel.next(!this.tcSvc.filterPanel.getValue());
  }
}
