import { HttpClient } from '@angular/common/http';
import { Injectable } from '@angular/core';
import { IncidentRecord } from '../incidents';
import { Observable } from 'rxjs';

export interface IncidentsListParams {
  start: Date | null;
  end: Date | null;
  filter: string | null;
  dir: string;
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

  createIncident(inp: IncidentRecord): Observable<IncidentRecord> {
    return this.http.post<IncidentRecord>('/api/incident/new', inp);
  }

  addRemoveCalls(id: string, inp: CallIncidentParams): Observable<void> {
    return this.http.post<void>('/api/incident/' + id + '/calls', inp);
  }

  deleteIncident(id: string): Observable<void> {
    return this.http.delete<void>('/api/incident/' + id);
  }

  updateIncident(id: string, inp: IncidentRecord): Observable<IncidentRecord> {
    return this.http.patch<IncidentRecord>('/api/incident/' + id, inp);
  }
}
