import { Injectable } from '@angular/core';
import { HttpClient, HttpResponse } from '@angular/common/http';
import { Observable, ReplaySubject } from 'rxjs';

export interface Preferences {
  tgsPerPage: number;
}

@Injectable({
  providedIn: 'root',
})
export class PrefsService {
  constructor(private http: HttpClient) {
    this.fetch();
  }
  last = <Preferences>{};
  private prefs = new ReplaySubject<Preferences>(1);
  prefs$: Observable<Preferences> = this.prefs.asObservable();

  fetch() {
    this.http.get<Preferences>('/api/user/prefs/stillbox').subscribe((res) => {
      if (res != null) {
        this.last = res;
      }
      this.prefs.next(res);
    });
  }

  setTGsPerPage(pp: number) {
    if (this.last.tgsPerPage != pp) {
      this.last.tgsPerPage = pp;
      this.setPrefs(this.last);
    }
  }

  setPrefs(p: Preferences) {
    this.http
      .put<Preferences>('/api/user/prefs/stillbox', p)
      .subscribe((event) => {});
    this.prefs.next(p);
  }
}
