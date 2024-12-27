import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import {
  BehaviorSubject,
  concatMap,
  Observable,
  ReplaySubject,
  share,
  shareReplay,
  Subscription,
  switchMap,
} from 'rxjs';

export interface Preferences {
  [key: string]: any;
}

function mapToObj(map: Map<any, any>): { [key: string]: any } {
  const obj: { [key: string]: any } = {};
  map.forEach((value, key) => {
    obj[key] = value;
  });
  return obj;
}

@Injectable({
  providedIn: 'root',
})
export class PrefsService {
  private readonly _getPref = new Map<string, ReplaySubject<any>>();
  prefs$: Observable<Preferences>;
  last!: Preferences;
  subscriptions = new Subscription();

  constructor(private http: HttpClient) {
    this.prefs$ = this.fetch().pipe(shareReplay(1));
    this.fillPrefs();
  }

  ngOnDestroy() {
    this.subscriptions.unsubscribe();
  }

  fillPrefs() {
    this.subscriptions.add(
      this.prefs$.subscribe((prefs) => {
        if (prefs) {
          this.last = prefs;
          Object.entries(prefs).forEach((pref) => {
            const rs = this._getPref.get(pref[0]);
            if (rs) {
              (rs as ReplaySubject<any>).next(pref[1]);
            } else {
              const bs = new ReplaySubject<any>(1);
              bs.next(pref[1]);
              this._getPref.set(pref[0], bs);
            }
          });
        } else {
          this.last = {};
        }
      }),
    );
  }

  fetch(): Observable<Preferences> {
    return this.http.get<Preferences>('/api/user/prefs/stillbox');
  }

  get(k: string): Observable<any> {
    if (!this._getPref.get(k)) {
      return this.prefs$.pipe(
        switchMap((pref) => {
          let ns = new ReplaySubject<any>(1);
          ns.next(pref ? pref[k] : null);
          return ns;
        }),
      );
    }

    return this._getPref.get(k)!;
  }

  set(pref: string, value: any) {
    this.last[pref] = value;
    let ex = this._getPref.get(pref);
    if (!ex) {
      ex = new ReplaySubject<any>(1);
      this._getPref.set(pref, ex);
    }
    this.http
      .put<Preferences>('/api/user/prefs/stillbox', this.last)
      .subscribe((ev) => {});
  }
}
