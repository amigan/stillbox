import { TestBed } from '@angular/core/testing';

import { ToolbarContextService } from './toolbar-context.service';

describe('ToolbarContextService', () => {
  let service: ToolbarContextService;

  beforeEach(() => {
    TestBed.configureTestingModule({});
    service = TestBed.inject(ToolbarContextService);
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });
});
