# Third-Party Software Licenses and Notices

This application bundles or uses the following open-source components:

## 1. Go-Native Dependencies (MIT & BSD Licenses)

These libraries are permissive and allow closed-source and commercial distribution, requiring only the inclusion of copyright notices.

### Walk GUI Library (`github.com/lxn/walk`)
*   **License:** BSD 3-Clause
*   **Copyright:** Copyright (c) 2010, The Walk Authors. All rights reserved.

### PDF Reader (`github.com/ledongthuc/pdf`)
*   **License:** BSD 3-Clause
*   **Copyright:** Copyright (c) 2018 Le Dong Thuc. All rights reserved.

### Excelize (`github.com/xuri/excelize/v2`)
*   **License:** BSD 3-Clause
*   **Copyright:** Copyright (c) 2016-2026 360 Enterprise Security Group. All rights reserved.

### HTML to Markdown (`github.com/JohannesKaufmann/html-to-markdown/v2`)
*   **License:** MIT License
*   **Copyright:** Copyright (c) 2018 Johannes Kaufmann.

### Docx to Markdown (`github.com/zakahan/docx2md`)
*   **License:** MIT License
*   **Copyright:** Copyright (c) 2019 zakahan.

---

## 2. Bundled Binaries (GPL License)

### Pandoc (`pandoc.exe`)
*   **License:** GNU General Public License v2.0 or later (GPLv2+)
*   **Source Code:** The source code of Pandoc is publicly available on GitHub at [https://github.com/jgm/pandoc](https://github.com/jgm/pandoc).
*   **Notices:** This application embeds a compiled binary of Pandoc (`pandoc.exe`) to handle advanced file conversions (PPTX, RTF, EPUB, LaTeX, etc.). Pandoc is extracted at runtime and executed as an independent subprocess communicating over standard process boundaries (command line arguments, pipes, stdin/stdout).
