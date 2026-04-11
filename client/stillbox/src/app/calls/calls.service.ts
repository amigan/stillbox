import { Injectable, Pipe, PipeTransform } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { BehaviorSubject, map, Observable, Subject } from 'rxjs';
import { CallRecord, CallStats } from '../calls';
import { environment } from '.././../environments/environment';
import { TalkgroupService } from '../talkgroups/talkgroups.service';
import { Talkgroup } from '../talkgroup';
import { Share } from '../shares';
import {
  DomSanitizer,
  SafeHtml,
  SafeResourceUrl,
  SafeScript,
  SafeStyle,
  SafeUrl,
} from '@angular/platform-browser';
import { PlayerService } from './player/player.service';

@Pipe({
  name: 'grabDate',
  standalone: true,
  pure: true,
})
export class DatePipe implements PipeTransform {
  transform(ts: string | Date, args?: any): string {
    const timestamp = new Date(ts);
    return timestamp.getMonth() + 1 + '/' + timestamp.getDate();
  }
}

export interface Talker {
  talkerAlias: string | null;
  source: number | null;
}

@Pipe({
  name: 'talker',
  standalone: true,
  pure: true,
})
export class TalkerPipe implements PipeTransform {
  transform(call: CallRecord, args?: any): Talker {
    let r = <Talker>{
      talkerAlias: call.talkerAlias,
      source: call.source,
    };

    return r;
  }
}

@Pipe({
  name: 'transcript',
  standalone: true,
  pure: true,
})
export class TranscriptPipe implements PipeTransform {
  transform(call: CallRecord, args?: any): string | null {
    return call.transcript;
  }
}

@Pipe({
  name: 'time',
  standalone: true,
  pure: true,
})
export class TimePipe implements PipeTransform {
  transform(ts: string | Date, haveSecond: boolean = false): string {
    const timestamp = new Date(ts);
    return timestamp.toLocaleTimeString(navigator.language, {
      hour: '2-digit',
      minute: '2-digit',
      second: haveSecond ? '2-digit' : undefined,
      hourCycle: 'h23',
    });
  }
}

@Pipe({
  name: 'talkgroup',
  standalone: true,
  pure: true,
})
export class TalkgroupPipe implements PipeTransform {
  constructor(private tgService: TalkgroupService) {}

  transform(
    call: CallRecord,
    field: string,
    share: Share | null = null,
  ): Observable<string> {
    return this.tgService.getTalkgroup(call.systemId, call.tgid).pipe(
      map((tg: Talkgroup) => {
        switch (field) {
          case 'alpha': {
            return tg.alphaTag ?? call.tgid;
            break;
          }
          case 'group': {
            return tg.tgGroup ?? '\u2014';
            break;
          }
          case 'system': {
            return tg.system?.name ?? tg.systemId.toString();
          }
          case 'summary': {
            let from = '';
            if (call.talkerAlias) {
              from = ` from ${call.talkerAlias}`;
            }
            let r = `To ${tg.alphaTag ?? call.tgid}${from}`;
            return r;
          }
          default: {
            return tg.name ?? '\u2014';
            break;
          }
        }
      }),
    );
  }
}

@Pipe({
  name: 'fixedPoint',
  standalone: true,
  pure: true,
})
export class FixedPointPipe implements PipeTransform {
  constructor() {}

  transform(quant: number, divisor: number, places: number): string {
    const seconds = quant / divisor;
    return seconds.toFixed(places);
  }
}

/**
 * Sanitize HTML
 */
@Pipe({
  name: 'safe',
})
export class SafePipe implements PipeTransform {
  /**
   * Pipe Constructor
   *
   * @param _sanitizer: DomSanitezer
   */
  // tslint:disable-next-line
  constructor(protected _sanitizer: DomSanitizer) {}

  /**
   * Transform
   *
   * @param value: string
   * @param type: string
   */
  transform(
    value: string,
    type: string,
  ): SafeHtml | SafeStyle | SafeScript | SafeUrl | SafeResourceUrl {
    switch (type) {
      case 'html':
        return this._sanitizer.bypassSecurityTrustHtml(value);
      case 'style':
        return this._sanitizer.bypassSecurityTrustStyle(value);
      case 'script':
        return this._sanitizer.bypassSecurityTrustScript(value);
      case 'url':
        return this._sanitizer.bypassSecurityTrustUrl(value);
      case 'resourceUrl':
        let res = this._sanitizer.bypassSecurityTrustResourceUrl(value);
        return res;
      default:
        return this._sanitizer.bypassSecurityTrustHtml(value);
    }
  }
}

@Pipe({
  name: 'audioDownloadURL',
  standalone: true,
  pure: true,
})
export class DownloadURLPipe implements PipeTransform {
  constructor(private callsSvc: CallsService) {}

  transform(call: CallRecord, args?: any): string {
    return this.callsSvc.callAudioDownloadURL(call.id);
  }
}

export interface CallsListParams {
  start: Date | null;
  end: Date | null;
  tagsAny: string[] | null;
  tagsNot: string[] | null;
  dir: string;
  page: number;
  perPage: number;
  tgFilter: string | null;
  sourceFilter: string | null;
  atLeastSeconds: number | null;
  transcriptSearch: string | null;
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
  public curLen = 0;

  getCalls(p: CallsListParams): Observable<CallsPaginated> {
    return this.http.post<CallsPaginated>('/api/call/', p);
  }

  callAudioURL(id: string, share: Share | undefined): string {
    if (share) {
      return environment.baseUrl + `/share/${share.id}/call/${id}`;
    }
    return environment.baseUrl + '/api/call/' + id;
  }

  callAudioDownloadURL(id: string): string {
    return environment.baseUrl + '/api/call/' + id + '/download';
  }

  getCallStats(interval: string): Observable<CallStats> {
    return this.http.get<CallStats>(`/api/call/stats/${interval}`);
  }
}
