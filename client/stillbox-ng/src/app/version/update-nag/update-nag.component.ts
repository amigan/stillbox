import { Component } from '@angular/core';
import { CheckerService } from '../checker.service';

@Component({
  selector: 'app-update-nag',
  imports: [],
  templateUrl: './update-nag.component.html',
  styleUrl: './update-nag.component.scss',
})
export class UpdateNagComponent {
  constructor(public checkerSvc: CheckerService) {}

  update() {
    this.checkerSvc.doUpdate();
  }
}
