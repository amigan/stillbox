import { Injectable, signal } from '@angular/core';
import { fromEvent, Observable, Subscription } from 'rxjs';
import { CallRecord } from '../../calls';
import { CallsService } from '../calls.service';
import { ErrorsService } from '../../errors/errors.service';
import { Params } from '@angular/router';

export type QueueOrigin =
  | { type: 'calls'; queryParams: Params }
  | { type: 'incident'; id: string };

interface IStack<T> {
  push(e: T[]): void;
  pop(): T | undefined;
  size(): number;
  cancel(): void;
}

class Stack<T> implements IStack<T> {
  private storage: T[] = [];

  constructor() {}

  push(items: T[]): void {
    this.storage.push(...items);
  }
  pop(): T | undefined {
    return this.storage.pop();
  }
  size(): number {
    return this.storage.length;
  }
  cancel(): void {
    this.storage = [];
  }
  shift(): T | undefined {
    return this.storage.shift();
  }
}

@Injectable({
  providedIn: 'root',
})
export class PlayerService {
  playSub!: Subscription;
  public playing = signal<CallRecord | null>(null);
  public paused = signal<boolean>(false);
  public queueOrigin = signal<QueueOrigin | null>(null);
  private pendingOrigin: QueueOrigin | null = null;
  stack = new Stack<CallRecord>();
  private results = <CallRecord[]>[];
  private forward = false;
  au = new Audio();

  constructor(
    private callsSvc: CallsService,
    private errorSvc: ErrorsService,
  ) {
    this.createMedia();
  }

  createMedia() {
    this.playSub = fromEvent(this.au, 'ended').subscribe((ev) => {
      this.playNext();
    });
  }

  setQueue(c: CallRecord[], origin?: QueueOrigin) {
    this.results = c;
    if (origin !== undefined) {
      this.pendingOrigin = origin;
    }
  }

  playNext() {
    if (this.stack.size() > 0) {
      let item = this.forward ? this.stack.shift() : this.stack.pop();
      if (item) {
        this.play(item);
      }
    } else {
      this.clearAudioState();
    }
  }

  /** Stops playback, clears the play stack and origin. Keeps the queue (results) so playback can be restarted. */
  stop() {
    this.clearAudioState();
    this.stack.cancel();
    this.queueOrigin.set(null);
    this.pendingOrigin = null;
  }

  private clearAudioState() {
    this.playing.set(null);
    this.paused.set(false);
    this.au.pause();
  }

  pauseAudio() {
    this.au.pause();
    this.paused.set(true);
  }

  playAudio(call: CallRecord, index: number, forward: boolean) {
    if (this.playing() != null) {
      this.clearAudioState();
    }
    this.queueOrigin.set(this.pendingOrigin);
    this.forward = forward;
    this.stack.cancel();
    let playq = this.forward
      ? this.results.slice(index)
      : this.results.slice(0, index + 1);
    this.stack.push(playq);
    this.playNext();
  }

  resume() {
    this.au.play();
    this.paused.set(false);
  }

  play(call: CallRecord) {
    this.paused.set(false);
    if (call.audioURL != null) {
      this.au.src = call.audioURL;
    } else {
      this.au.src = this.callsSvc.callAudioURL(call.id);
    }
    this.au.load();
    this.au
      .play()
      .then(() => {
        this.playing.set(call);
      })
      .catch((reason) => {
        // cannot figure out why we need to do this
        if (
          reason instanceof Error &&
          (reason as Error).message.indexOf(
            'media was removed from the document',
          ) > -1
        ) {
          this.playing.set(call);
          return;
        }
        this.playing.set(null);
        this.errorSvc.show(`play failed: ${reason}`);
      });
  }
}
