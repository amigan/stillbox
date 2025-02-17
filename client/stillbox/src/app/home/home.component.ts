import { Component } from '@angular/core';
import { ChartsComponent } from '../charts/charts.component';
import { MatCardModule } from '@angular/material/card';

@Component({
  selector: 'app-home',
  imports: [ChartsComponent, MatCardModule],
  templateUrl: './home.component.html',
  styleUrl: './home.component.scss',
})
export class HomeComponent {}
