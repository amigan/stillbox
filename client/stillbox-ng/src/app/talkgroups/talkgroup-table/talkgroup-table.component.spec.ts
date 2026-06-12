import { ComponentFixture, TestBed } from '@angular/core/testing';

import { TalkgroupTableComponent } from './talkgroup-table.component';

describe('TalkgroupTableComponent', () => {
  let component: TalkgroupTableComponent;
  let fixture: ComponentFixture<TalkgroupTableComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [TalkgroupTableComponent],
    }).compileComponents();

    fixture = TestBed.createComponent(TalkgroupTableComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
