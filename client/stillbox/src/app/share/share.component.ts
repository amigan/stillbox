import { Component } from '@angular/core';
import { ShareService } from './share.service';
import { Share } from '../shares';
import { ActivatedRoute } from '@angular/router';
import { Observable, Subscription, switchMap } from 'rxjs';
import { IncidentComponent } from '../incidents/incident/incident.component';
import { AsyncPipe } from '@angular/common';
import { CallInfoComponent } from '../calls/call-info/call-info.component';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';

@Component({
  selector: 'app-share',
  imports: [
    AsyncPipe,
    IncidentComponent,
    CallInfoComponent,
    MatProgressSpinnerModule,
  ],
  templateUrl: './share.component.html',
  styleUrl: './share.component.scss',
})
export class ShareComponent {
  shareID!: string;
  share!: Observable<Share>;
  constructor(
    private route: ActivatedRoute,
    private shareSvc: ShareService,
  ) {}

  ngOnInit() {
    let shareParam = this.route.snapshot.paramMap.get('id');
    if (shareParam == null) {
      // TODO: error
      return;
    }

    this.shareID = shareParam;

    this.share = this.shareSvc.getShare(this.shareID);
  }
}
