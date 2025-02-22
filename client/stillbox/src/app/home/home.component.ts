import { Component, computed, signal, ViewChild } from '@angular/core';
import { ChartsComponent } from '../charts/charts.component';
import { MatCardModule } from '@angular/material/card';
import { FormControl, FormGroup, FormsModule, ReactiveFormsModule } from '@angular/forms';
import { debounceTime, filter, Observable, startWith, switchAll, switchMap } from 'rxjs';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatSelectModule } from '@angular/material/select';
import { MatInputModule } from '@angular/material/input';

@Component({
  selector: 'app-home',
  imports: [ChartsComponent, MatCardModule, MatFormFieldModule, MatSelectModule, MatInputModule, FormsModule, ReactiveFormsModule ],
  templateUrl: './home.component.html',
  styleUrl: './home.component.scss',
})
export class HomeComponent {
  form = new FormGroup({
    callsPer: new FormControl(),
  });
  callsOb!: Observable<string>;

  ngOnInit() {
    this.form.controls["callsPer"].setValue("week");
    this.callsOb = this.form.controls['callsPer'].valueChanges
    .pipe(
      startWith("week"),
      debounceTime(300),
      filter(val => val !== null)
    );
  }
}
