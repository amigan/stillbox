import { Component } from '@angular/core';
import { ShareService } from './share.service';
import { ActivatedRoute } from '@angular/router';

@Component({
  selector: 'app-share',
  imports: [],
  templateUrl: './share.component.html',
  styleUrl: './share.component.scss'
})
export class ShareComponent {
  constructor(
    private route: ActivatedRoute,
    private shareSvc: ShareService,
  ) {}

  ngOnInit() {

  }
}
