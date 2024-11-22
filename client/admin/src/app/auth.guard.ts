import { Router, CanActivateFn } from '@angular/router';
import { AuthService } from './login/auth.service';
import { inject } from '@angular/core';

export const AuthGuard: CanActivateFn = (route, state) => {
  const router: Router = inject(Router);
  const authSvc: AuthService = inject(AuthService);
  if (sessionStorage.getItem('jwt') == null) {
    let success = false;
    authSvc.refresh().subscribe((event) => {
      if (event?.status == 200) {
        success = true;
      }
    });
    router.navigate(['/login']);
    return success;
  } else {
    return true;
  }
};
