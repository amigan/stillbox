import { Component, inject } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { MatToolbarModule } from '@angular/material/toolbar';

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
  RouterLink,
} from '@angular/router';
import { filter, map } from 'rxjs';
import { NowPlayingComponent } from './calls/now-playing/now-playing.component';
import { ErrorsComponent } from './errors/errors.component';
import { AvatarComponent } from './user/avatar/avatar.component';
import { MatMenuModule } from '@angular/material/menu';

@Component({
  selector: 'app-root',
  imports: [
    RouterModule,
    MatToolbarModule,
    FormsModule,
    NavigationComponent,
    UpdateNagComponent,
    ErrorsComponent,
    AvatarComponent,
    MatMenuModule,
    NowPlayingComponent,
    RouterModule,
    RouterLink,
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
  isLoginPage = false;

  constructor(
    private router: Router,
    private titleService: Title,
  ) {}

  toggleNav() {
    this.toggleNavSubject.next();
  }

  ngOnInit() {
    this.isLoginPage = this.router.url.startsWith('/login');
    this.titleSub = this.router.events
      .pipe(
        filter((event) => event instanceof NavigationEnd),
        map(() => {
          this.isLoginPage = this.router.url.startsWith('/login');
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
