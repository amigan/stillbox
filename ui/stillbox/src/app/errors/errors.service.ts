import { Injectable, signal } from '@angular/core';

@Injectable({
  providedIn: 'root',
})
export class ErrorsService {
  currentError = signal<string | null>(null);
  constructor() {}

  show(s: string) {
    this.currentError.set(s);
    setTimeout(() => {
      this.currentError.set(null);
    }, 4000);
  }
}
