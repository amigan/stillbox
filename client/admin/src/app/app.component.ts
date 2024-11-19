import { Component, inject } from '@angular/core';
import { RouterModule, RouterOutlet, RouterLink } from '@angular/router';
import { CommonModule } from '@angular/common';
import { AuthService } from './login/auth.service';
import { NgIconComponent, provideIcons } from '@ng-icons/core';
import {
  ionMenuOutline,
  ionChatbubbles,
  ionNewspaperOutline,
  ionAlertCircleOutline,
  ionRadioOutline,
  ionHome,
  ionMegaphoneOutline,
  ionCreateOutline,
} from '@ng-icons/ionicons';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [
    CommonModule,
    RouterOutlet,
    RouterModule,
    RouterLink,
    NgIconComponent,
  ],
  templateUrl: './app.component.html',
  styleUrl: './app.component.css',
  providers: [
    provideIcons({
      ionMenuOutline,
      ionChatbubbles,
      ionNewspaperOutline,
      ionAlertCircleOutline,
      ionRadioOutline,
      ionHome,
      ionMegaphoneOutline,
      ionCreateOutline,
    }),
  ],
})
export class AppComponent {
  auth: AuthService = inject(AuthService);
  title = 'admin';

  logout() {
    this.auth.logout();
  }
}
