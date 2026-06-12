import {
  ReactiveFormsModule,
  FormGroup,
  FormControl,
  FormsModule,
} from '@angular/forms';
import { TalkgroupService } from '../talkgroups.service';
import { Component, inject, Pipe, PipeTransform } from '@angular/core';

@Component({
  selector: 'app-export',
  imports: [ReactiveFormsModule, FormsModule],
  templateUrl: './export.component.html',
  styleUrl: './export.component.scss',
})
export class ExportComponent {
  tgService: TalkgroupService = inject(TalkgroupService);
  file!: File;
  form!: FormGroup;

  ngOnInit() {
    this.form = new FormGroup({
      sysID: new FormControl(),
      template: new FormControl(),
    });
  }

  submit() {
    this.tgService
      .exportTGs(
        'sdrtrunk',
        Number(this.form.controls['sysID'].value),
        this.form.controls['template'].value,
      )
      .subscribe((response) => {
        let fileName = '';
        const contentDisposition = response.headers.get('Content-Disposition');
        if (contentDisposition) {
          const fileNameRegex = /filename[^;=\n]*=((['"]).*?\2|[^;\n]*)/;
          const matches = fileNameRegex.exec(contentDisposition);
          if (matches != null && matches[1]) {
            fileName = matches[1].replace(/['"]/g, '');
          }
        }
        let downloadLink = document.createElement('a');
        downloadLink.href = window.URL.createObjectURL(response.body!);
        if (fileName) downloadLink.setAttribute('download', fileName);
        document.body.appendChild(downloadLink);
        downloadLink.click();
      });
  }

  fileChange(files: Event) {
    let ft = files.target as HTMLInputElement;
    if (ft == null || ft.files == null) {
      return;
    }
    const file = ft.files[0]; // Here we use only the first file (single file)
    this.form.patchValue({ template: file });
  }
}
