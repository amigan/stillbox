import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';
import { CallRecord } from '../calls';
import { environment } from '.././../environments/environment';

export interface CallsListParams {
  start: Date | null;
  end: Date | null;
  tagsAny: string[] | null;
  tagsNot: string[] | null;
  dir: string;
  page: number;
  perPage: number;
  tgFilter: string | null;
  atLeastSeconds: number | null;
}

export interface CallsPaginated {
  calls: CallRecord[];
  count: number;
}

@Injectable({
  providedIn: 'root',
})
export class CallsService {
  constructor(private http: HttpClient) {}

  getCalls(p: CallsListParams): Observable<CallsPaginated> {
    return this.http.post<CallsPaginated>('/api/call/', p);
  }

  callAudioURL(id: string): string {
    return environment.baseUrl + '/api/call/' + id;
  }

  callAudioDownloadURL(id: string): string {
    return environment.baseUrl + '/api/call/' + id + '/download';
  }
}
