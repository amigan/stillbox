import { CanActivateFn } from '@angular/router';

export const AuthGuard: CanActivateFn = (route, state) => {
  if (sessionStorage.getItem('jwt') == null) {
    return false;
  } else {
    return true;
  }
};
