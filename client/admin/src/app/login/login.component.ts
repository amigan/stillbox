import { Component, inject } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { AuthService } from '../login/auth.service';
import { catchError, of } from 'rxjs';
import { Router } from '@angular/router';

@Component({
  selector: 'app-login',
  standalone: true,
  imports: [FormsModule],
  templateUrl: './login.component.html',
  styleUrl: './login.component.css',
})
export class LoginComponent {
  apiService: AuthService = inject(AuthService);
  router: Router = inject(Router);
  username: string = '';
  password: string = '';
  failed: boolean = false;

  onSubmit() {
    this.failed = false;
    this.apiService
      .login(this.username, this.password)
      .pipe(
        catchError(() => {
          this.failed = true;
          return of(null);
        }),
      )
      .subscribe((event) => {
        if (event?.status == 200) {
          this.router.navigateByUrl('/home');
        } else {
          this.failed = true;
        }
      });
  }
}
