import { Component, inject } from '@angular/core';
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
  failed: boolean = false;
  private subscriptions = new Subscription();

  onSubmit() {
    this.failed = false;
    this.subscriptions.add(
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
            this.router.navigateByUrl('/');
          } else {
            this.failed = true;
          }
        }),
    );
  }

  ngOnDestroy() {
    this.subscriptions.unsubscribe();
  }
}
