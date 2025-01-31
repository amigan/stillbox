import { HttpClient } from '@angular/common/http';
import { Injectable } from '@angular/core';
import { map, Observable, switchMap } from 'rxjs';
import { IncidentRecord } from '../incidents';

type ShareType = IncidentRecord | ArrayBuffer;
export interface Share {
  shareType: string;
  share: ShareType;
}
@Injectable({
  providedIn: 'root'
})
export class ShareService {

  constructor(
    private http: HttpClient,
  ) { }

  getShare(id: string): Observable<Share|null> {
    return this.http.get<ShareType>(`/share/${id}`, {observe: 'response'}).pipe(
        map((res) => {
          let typ = res.headers.get('X-Share-Type');
          switch(typ) {
            case 'call':
              return <Share>{shareType: typ, share: (res.body as ArrayBuffer)};
            case 'incident':
              return <Share>{shareType: typ, share: (res.body as IncidentRecord)};
          }
          return null;
        })
      );
  }
}
