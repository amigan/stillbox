import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';
import { Talkgroup, TalkgroupUpdate } from '../talkgroup';

@Injectable({
  providedIn: 'root',
})

export interface Pagination {
  page: number;
  perPage: number;
}

export class TalkgroupService {
  loggedIn: boolean = false;
  constructor(private http: HttpClient) {}

  getTalkgroups(pagination?: Pagination): Observable<Talkgroup[]> {
    if (pagination == null) {
      return this.http.get<Talkgroup[]>('/api/talkgroup/');
    }

    return this.http.post<Talkgroup[]>('/api/talkgroup/', pagination);
  }

  getTalkgroup(sys: number, tg: number): Observable<Talkgroup> {
    return this.http.get<Talkgroup>(`/api/talkgroup/${sys}/${tg}`);
  }

  importRR(sysID: number, content: string): Observable<Talkgroup[]> {
    return this.http.post<Talkgroup[]>('/api/talkgroup/import', {
      systemID: sysID,
      type: 'radioreference',
      body: content,
    });
  }

  putTalkgroup(tu: TalkgroupUpdate): Observable<Talkgroup> {
    return this.http.put<Talkgroup>(
      `/api/talkgroup/${tu.system_id}/${tu.tgid}`,
      tu,
    );
  }

  putTalkgroups(
    sysID: Number,
    tgs: TalkgroupUpdate[],
  ): Observable<Talkgroup[]> {
    return this.http.put<Talkgroup[]>(`/api/talkgroup/${sysID}`, tgs);
  }
}
