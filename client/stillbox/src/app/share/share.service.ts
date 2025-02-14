import { HttpClient } from '@angular/common/http';
import { Injectable } from '@angular/core';
import { map, Observable, switchMap } from 'rxjs';
import { IncidentRecord } from '../incidents';
import { CallRecord } from '../calls';
import { Share, ShareType } from '../shares';
import { ActivatedRoute, Router } from '@angular/router';

export interface ShareRecord {
  id: string;
  entityType: string;
  entityDate: Date;
  owner: string;
  entityID: string;
  expiration: Date | null;
}

export interface ShareListParams {
  page: number | null;
  perPage: number | null;
  dir: string | null;
}

export interface Shares {
  shares: ShareRecord[];
  totalCount: number;
}

@Injectable({
  providedIn: 'root',
})
export class ShareService {
  constructor(
    private http: HttpClient,
    private router: Router,
    private route: ActivatedRoute,
  ) {}

  inShare(): string | null {
    if (this.router.url.startsWith('/s/')) {
      return this.route.snapshot.paramMap.get('id');
    }

    return null;
  }

  getShare(id: string): Observable<Share> {
    return this.http.get<Share>(`/share/${id}`);
  }

  deleteShare(id: string): Observable<void> {
    return this.http.delete<void>(`/api/share/${id}`);
  }

  getShares(p: ShareListParams): Observable<Shares> {
    return this.http.post<Shares>('/api/share/', p);
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
