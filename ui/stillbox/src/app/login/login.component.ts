import { Component, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { AuthService } from '../login/auth.service';
import { catchError, of, Subscription } from 'rxjs';
import { Router } from '@angular/router';
import { MatInputModule } from '@angular/material/input';
import { MatFormFieldModule } from '@angular/material/form-field';

@Component({
  selector: 'app-login',
  imports: [FormsModule, MatInputModule, MatFormFieldModule],
  templateUrl: './login.component.html',
  styleUrl: './login.component.scss',
})
export class LoginComponent {
  apiService: AuthService = inject(AuthService);
  router: Router = inject(Router);
  username: string = '';
  password: string = '';
  failed = this.apiService.loginFailed;

  onSubmit() {
    this.apiService.login(this.username, this.password);
  }
}
