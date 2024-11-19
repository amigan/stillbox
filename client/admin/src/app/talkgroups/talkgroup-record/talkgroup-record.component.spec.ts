import { ComponentFixture, TestBed } from '@angular/core/testing';

import { TalkgroupRecordComponent } from './talkgroup-record.component';

describe('TalkgroupRecordComponent', () => {
  let component: TalkgroupRecordComponent;
  let fixture: ComponentFixture<TalkgroupRecordComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [TalkgroupRecordComponent],
    }).compileComponents();

    fixture = TestBed.createComponent(TalkgroupRecordComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
