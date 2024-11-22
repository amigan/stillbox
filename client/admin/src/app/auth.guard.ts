import { Router, CanActivateFn } from '@angular/router';
import { inject } from '@angular/core';

export const AuthGuard: CanActivateFn = (route, state) => {
  const router: Router = inject(Router);
  if (sessionStorage.getItem('jwt') == null) {
    router.navigate(['/login']);
    return false;
  } else {
    return true;
  }
};
