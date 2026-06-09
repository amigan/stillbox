import { Component } from '@angular/core';
import { ErrorsService } from './errors.service';

@Component({
  selector: 'app-error-bar',
  imports: [],
  templateUrl: './errors.component.html',
  styleUrl: './errors.component.scss',
})
export class ErrorsComponent {
  constructor(public errorSvc: ErrorsService) {}
}
