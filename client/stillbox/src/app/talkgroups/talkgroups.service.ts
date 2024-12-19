import { Injectable } from '@angular/core';
import { HttpClient, HttpResponse } from '@angular/common/http';
import { Observable } from 'rxjs';
import { Talkgroup, TalkgroupUpdate } from '../talkgroup';

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
  constructor(private http: HttpClient) {}

  getTalkgroups(): Observable<Talkgroup[]> {
    return this.http.get<Talkgroup[]>('/api/talkgroup/');
  }

  getTalkgroupsPag(pagination: Pagination): Observable<TalkgroupsPaginated> {
    return this.http.post<TalkgroupsPaginated>('/api/talkgroup/', pagination);
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

  allTags(): Observable<string[]> {
    return this.http.get<string[]>('/api/talkgroup/tags');
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
