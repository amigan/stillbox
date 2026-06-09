import { HttpClient, HttpHeaders, HttpParams } from '@angular/common/http';
import { Injectable } from '@angular/core';
import { IncidentCalls, IncidentRecord } from '../incidents';
import { Observable } from 'rxjs';

export interface IncidentsListParams {
  start: Date | null;
  end: Date | null;
  filter: string | null;
  dir: 'asc' | 'desc';
  orderBy: 'start' | 'numcalls';
  page: number;
  perPage: number;
}

export interface IncidentCallsParams {
  page: number;
  perPage: number;
}

export interface CallIncidentParams {
  add: string[] | null; // IDs
  notes: Object | null;
  remove: string[] | null; // IDs
}

export interface IncidentsPaginated {
  incidents: IncidentRecord[];
  count: number;
}
@Injectable({
  providedIn: 'root',
})
export class IncidentsService {
  constructor(private http: HttpClient) {}

  getIncidents(p: IncidentsListParams): Observable<IncidentsPaginated> {
    return this.http.post<IncidentsPaginated>('/api/incident/', p);
  }

  getIncidentCalls(
    id: string,
    p: IncidentCallsParams,
  ): Observable<IncidentCalls> {
    return this.http.post<IncidentCalls>('/api/incident/' + id + '/calls', p);
  }

  /** Public share link: GET with query params (backend reads these via urlencoded form parse). */
  getSharedIncidentCalls(
    shareId: string,
    p: IncidentCallsParams,
  ): Observable<IncidentCalls> {
    return this.http.post<IncidentCalls>(`/share/${shareId}/calls`, p);
  }

  createIncident(inp: IncidentRecord): Observable<IncidentRecord> {
    return this.http.post<IncidentRecord>('/api/incident/new', inp);
  }

  addRemoveCalls(id: string, inp: CallIncidentParams): Observable<void> {
    return this.http.patch<void>('/api/incident/' + id + '/calls', inp);
  }

  deleteIncident(id: string): Observable<void> {
    return this.http.delete<void>('/api/incident/' + id);
  }

  updateIncident(id: string, inp: IncidentRecord): Observable<IncidentRecord> {
    return this.http.patch<IncidentRecord>('/api/incident/' + id, inp);
  }

  getIncident(id: string): Observable<IncidentRecord> {
    return this.http.get<IncidentRecord>('/api/incident/' + id);
  }
}
