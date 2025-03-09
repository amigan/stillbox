import { Component, inject } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { MatSidenavModule } from '@angular/material/sidenav';
import { MatToolbarModule } from '@angular/material/toolbar';
import { CommonModule } from '@angular/common';
import { AuthService } from './login/auth.service';
import { NavigationComponent } from './navigation/navigation.component';
import { Subject, Subscription } from 'rxjs';
import { Title } from '@angular/platform-browser';
import { UpdateNagComponent } from './version/update-nag/update-nag.component';
import {
  Router,
  RouterModule,
  NavigationEnd,
  ActivatedRoute,
} from '@angular/router';
import { filter, map } from 'rxjs';
import { NowPlayingComponent } from './calls/now-playing/now-playing.component';

@Component({
  selector: 'app-root',
  imports: [
    CommonModule,
    RouterModule,
    MatToolbarModule,
    FormsModule,
    NavigationComponent,
    UpdateNagComponent,
    NowPlayingComponent,
  ],
  templateUrl: './app.component.html',
  styleUrls: ['./app.component.scss'],
})
export class AppComponent {
  auth: AuthService = inject(AuthService);
  title = 'stillbox';
  opened!: boolean;
  toggleNavSubject: Subject<void> = new Subject<void>();
  titleSub!: Subscription;

  constructor(
    private router: Router,
    private titleService: Title,
  ) {}

  toggleNav() {
    this.toggleNavSubject.next();
  }

  ngOnInit() {
    this.titleSub = this.router.events
      .pipe(
        filter((event) => event instanceof NavigationEnd),
        map(() => {
          let route: ActivatedRoute = this.router.routerState.root;
          let routeTitle = '';
          while (route!.firstChild) {
            route = route.firstChild;
          }
          if (route.snapshot.data['title']) {
            routeTitle = route!.snapshot.data['title'];
          }
          return routeTitle;
        }),
      )
      .subscribe((title: string) => {
        if (title) {
          this.titleService.setTitle(`${title} | Stillbox`);
        }
      });
  }

  logout() {
    this.titleSub.unsubscribe();
    this.auth.logout();
  }
}
