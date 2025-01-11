import { Injectable } from '@angular/core';
import { HttpClient, HttpResponse } from '@angular/common/http';
import {
  BehaviorSubject,
  concatMap,
  Observable,
  ReplaySubject,
  shareReplay,
  Subscription,
  switchMap,
} from 'rxjs';
import { Talkgroup, TalkgroupUpdate, TGID } from '../talkgroup';

export interface Pagination {
  page: number;
  perPage: number;
  filter: string | null;
}

export interface TalkgroupsPaginated {
  talkgroups: Talkgroup[];
  count: number;
}

@Injectable({
  providedIn: 'root',
})
export class TalkgroupService {
  private readonly _getTalkgroup = new Map<string, ReplaySubject<Talkgroup>>();
  private tgs$: Observable<Talkgroup[]>;
  private tags$!: Observable<string[]>;
  private fetchAll = new BehaviorSubject<'fetch'>('fetch');
  private subscriptions = new Subscription();
  constructor(private http: HttpClient) {
    this.tgs$ = this.fetchAll.pipe(switchMap(() => this.getTalkgroups()));
    this.tags$ = this.fetchAll.pipe(
      switchMap(() => this.getAllTags()),
      shareReplay(),
    );
    this.fillTgMap();
  }

  ngOnDestroy() {
    this.subscriptions.unsubscribe();
  }

  getAllTags(): Observable<string[]> {
    return this.http.get<string[]>('/api/talkgroup/tags').pipe(shareReplay());
  }

  getTalkgroups(): Observable<Talkgroup[]> {
    return this.http.get<Talkgroup[]>('/api/talkgroup/').pipe(shareReplay());
  }

  getTalkgroup(sys: number, tg: number): Observable<Talkgroup> {
    const key = this.tgKey(sys, tg);
    if (!this._getTalkgroup.get(key)) {
      let rs = new ReplaySubject<Talkgroup>();
      this._getTalkgroup.set(key, rs);
    }
    return this._getTalkgroup.get(key)!;
  }

  putTalkgroup(tu: TalkgroupUpdate): Observable<Talkgroup> {
    let tgid = this.tgKey(tu.system_id, tu.tgid);

    return this.http
      .put<Talkgroup>(`/api/talkgroup/${tu.system_id}/${tu.tgid}`, tu)
      .pipe(
        switchMap((tg) => {
          let tObs = this._getTalkgroup.get(tgid);
          if (!tObs) {
            tObs = new ReplaySubject<Talkgroup>(1);
            this._getTalkgroup.set(tgid, tObs);
          }

          tObs.next(tg);
          return tObs;
        }),
      );
  }

  putTalkgroups(
    sysID: Number,
    tgs: TalkgroupUpdate[],
  ): Observable<Talkgroup[]> {
    this._getTalkgroup.clear();
    return this.http.put<Talkgroup[]>(`/api/talkgroup/${sysID}`, tgs);
  }

  tgKey(sys: number, tg: number): string {
    return sys + ':' + tg;
  }

  getTalkgroupsPag(pagination: Pagination): Observable<TalkgroupsPaginated> {
    return this.http.post<TalkgroupsPaginated>('/api/talkgroup/', pagination);
  }

  fillTgMap() {
    this.subscriptions.add(
      this.tgs$.subscribe((tgs) => {
        tgs.forEach((tg) => {
          let tgid = this.tgKey(tg.system_id, tg.tgid);
          const rs = this._getTalkgroup.get(tgid);
          if (rs) {
            (rs as ReplaySubject<Talkgroup>).next(tg);
          } else {
            const bs = new ReplaySubject<Talkgroup>(1);
            bs.next(tg);
            this._getTalkgroup.set(tgid, bs);
          }
        });
      }),
    );
  }

  importRR(sysID: number, content: string): Observable<Talkgroup[]> {
    return this.http.post<Talkgroup[]>('/api/talkgroup/import', {
      systemID: sysID,
      type: 'radioreference',
      body: content,
    });
  }

  allTags(): Observable<string[]> {
    return this.tags$;
  }

  exportTGs(
    type: string,
    sysID: number,
    template: File,
  ): Observable<HttpResponse<Blob>> {
    let fd = new FormData();
    fd.append('type', type);
    fd.append('systemID', sysID.toString());
    fd.append('template', template);
    return this.http.post<Blob>('/api/talkgroup/export', fd, {
      observe: 'response',
      responseType: 'blob' as 'json',
    });
  }
}
