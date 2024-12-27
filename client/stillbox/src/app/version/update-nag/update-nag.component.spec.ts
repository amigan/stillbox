import { ComponentFixture, TestBed } from '@angular/core/testing';

import { UpdateNagComponent } from './update-nag.component';

describe('UpdateNagComponent', () => {
  let component: UpdateNagComponent;
  let fixture: ComponentFixture<UpdateNagComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [UpdateNagComponent],
    }).compileComponents();

    fixture = TestBed.createComponent(UpdateNagComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
