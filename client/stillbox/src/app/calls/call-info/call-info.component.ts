import { Component, Input } from '@angular/core';
import { MatCardModule } from '@angular/material/card';
import { CallRecord } from '../../calls';
import { Share } from '../../shares';
import { DatePipe, SafePipe, TalkgroupPipe, TimePipe } from '../calls.service';
import { AsyncPipe } from '@angular/common';

@Component({
  selector: 'app-call-info',
  imports: [
    MatCardModule,
    TimePipe,
    DatePipe,
    TalkgroupPipe,
    AsyncPipe,
    SafePipe,
  ],
  templateUrl: './call-info.component.html',
  styleUrl: './call-info.component.scss',
})
export class CallInfoComponent {
  @Input() share!: Share;
  call!: CallRecord;

  ngOnInit() {
    this.call = this.share.sharedItem as CallRecord;
  }
}
