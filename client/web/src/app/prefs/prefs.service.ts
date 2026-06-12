import { effect, Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import {
  Observable,
  of,
  ReplaySubject,
  shareReplay,
  Subscription,
  switchMap,
} from 'rxjs';
import { AuthService } from '../login/auth.service';

export interface UserSysPreferences {
  userPrefs: Preferences;
  sysPrefs: Preferences;
}
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
  prefs$: Observable<UserSysPreferences>;
  last!: UserSysPreferences;
  private prefsSub?: Subscription;

  private createEmptyPrefs(): UserSysPreferences {
    return {
      userPrefs: {},
      sysPrefs: {},
    };
  }

  constructor(
    private http: HttpClient,
    private authSvc: AuthService,
  ) {
    this.last = this.createEmptyPrefs();
    this.prefs$ = of(this.createEmptyPrefs()).pipe(shareReplay(1));

    effect(() => {
      if (this.authSvc.isAuth()) {
        this.prefs$ = this.fetch().pipe(shareReplay(1));
        this.fillPrefs();
      } else {
        this.last = this.createEmptyPrefs();
      }
    });
  }

  ngOnDestroy() {
    this.prefsSub?.unsubscribe();
  }

  fillPrefs() {
    this.prefsSub?.unsubscribe();
    this.prefsSub = this.prefs$.subscribe((prefs) => {
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
        this.last = <UserSysPreferences>{};
      }
    });
  }

  fetch(): Observable<UserSysPreferences> {
    return this.http.get<UserSysPreferences>('/api/prefs/stillbox');
  }

  get(k: string): Observable<any> {
    if (!this._getPref.get(k)) {
      return this.prefs$.pipe(
        switchMap((res) => {
          let rv = <Preferences>res.sysPrefs;
          let pref = <Preferences>mergeDeep(rv, res.userPrefs);
          let ns = new ReplaySubject<any>(1);
          ns.next(pref ? pref[k] : null);
          return ns;
        }),
      );
    }

    return this._getPref.get(k)!;
  }

  set(pref: string, value: any) {
    if (this.last == null) {
      this.last = <UserSysPreferences>{};
    }
    if (this.last.userPrefs == null) {
      this.last.userPrefs = <Preferences>{};
    }
    this.last.userPrefs[pref] = value;
    let ex = this._getPref.get(pref);
    if (ex == null) {
      ex = new ReplaySubject<any>(1);
      this._getPref.set(pref, ex);
      ex.next(value);
    }
    if (!this.authSvc.isAuth()) {
      return;
    }
    this.http
      .put<Preferences>('/api/prefs/stillbox', this.last.userPrefs)
      .subscribe((ev) => {});
  }
}

/**
 * Simple object check.
 * @param item
 * @returns {boolean}
 */
export function isObject(item: any): boolean {
  return item && typeof item === 'object' && !Array.isArray(item);
}

/**
 * Deep merge two objects.
 * @param target
 * @param ...sources
 */
export function mergeDeep(target: any, ...sources: any): any {
  if (!sources.length) return target;
  const source = sources.shift();

  if (isObject(target) && isObject(source)) {
    for (const key in source) {
      if (isObject(source[key])) {
        if (!target[key]) Object.assign(target, { [key]: {} });
        mergeDeep(target[key], source[key]);
      } else {
        Object.assign(target, { [key]: source[key] });
      }
    }
  }

  return mergeDeep(target, ...sources);
}
