import { Component } from '@angular/core';
import { Share, ShareService } from './share.service';
import { ActivatedRoute } from '@angular/router';
import { Observable, Subscription, switchMap } from 'rxjs';
import { IncidentComponent } from '../incidents/incident/incident.component';
import { AsyncPipe } from '@angular/common';

@Component({
  selector: 'app-share',
  imports: [AsyncPipe, IncidentComponent],
  templateUrl: './share.component.html',
  styleUrl: './share.component.scss',
})
export class ShareComponent {
  shareID!: string;
  share!: Observable<Share | null>;
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
