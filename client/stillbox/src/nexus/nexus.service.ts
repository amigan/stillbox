import { HttpClient } from '@angular/common/http';
import { Injectable } from '@angular/core';
import { WebSocketSubject, webSocket } from 'rxjs/webSocket';
import { Message } from '../proto/stillbox';

function genWSSURL(): string {
  var loc = window.location,
    newURI;
  if (loc.protocol == 'https') {
    newURI = 'wss:';
  } else {
    newURI = 'ws:';
  }

  newURI += '//' + loc.host + loc.pathname + '/api/ws';
  return newURI;
}

@Injectable({
  providedIn: 'root',
})
export class NexusService {
  private socket$: WebSocketSubject<Message>;
  constructor(private http: HttpClient) {
    this.socket$ = webSocket(genWSSURL());
  }
}
