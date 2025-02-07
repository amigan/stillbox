import { HttpClient } from '@angular/common/http';
import { Injectable } from '@angular/core';
import { map, Observable, switchMap } from 'rxjs';
import { IncidentRecord } from '../incidents';
import { CallRecord } from '../calls';
import { Share, ShareType } from '../shares';

@Injectable({
  providedIn: 'root',
})
export class ShareService {
  constructor(private http: HttpClient) {}

  getShare(id: string): Observable<Share> {
    return this.http.get<Share>(`/share/${id}`);
  }

  getSharedItem(s: Observable<Share>): Observable<ShareType> {
    return s.pipe(
      map((res) => {
        switch (res.type) {
          case 'call':
            return <CallRecord>res.sharedItem;
          case 'incident':
            return <IncidentRecord>res.sharedItem;
        }

        return null;
      }),
    );
  }

  getCallAudio(s: Observable<CallRecord>): Observable<ArrayBuffer> {
    return s.pipe(
      switchMap((res) => {
        return this.http.get<ArrayBuffer>(res.audioURL!);
      }),
    );
  }
}
