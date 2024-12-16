import { Component, inject } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { MatSidenavModule, MatSidenav } from '@angular/material/sidenav';
import { MatToolbarModule } from '@angular/material/toolbar';
import { RouterModule } from '@angular/router';
import { CommonModule } from '@angular/common';
import { AuthService } from './login/auth.service';
import { NavigationComponent } from './navigation/navigation.component';
import { Subject } from 'rxjs';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [
    CommonModule,
    RouterModule,
    MatSidenavModule,
    MatToolbarModule,
    FormsModule,
    NavigationComponent,
  ],
  templateUrl: './app.component.html',
  styleUrls: ['./app.component.scss'],
})
export class AppComponent {
  auth: AuthService = inject(AuthService);
  title = 'stillbox';
  opened!: boolean;
  toggleNavSubject: Subject<void> = new Subject<void>();

  toggleNav() {
    this.toggleNavSubject.next();
  }

  logout() {
    this.auth.logout();
  }
}
