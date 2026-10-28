## dl
Download an online video as an MP4 with yt-dlp.

### Why
Downloads a video as an MP4 that plays everywhere, without remembering yt-dlp's format and cookie options.

### Usage

```text
dl v2.8.0
Download an online video as an MP4 with yt-dlp
github.com/queone/gkit/tree/main/cmd/dl

Usage
  dl [options] FILENAME "URL"  Save the video at URL as FILENAME.mp4

  Downloads default to a small 360p (max 640x360, or 360x640 portrait) mp4 for quick sharing.
  A bare FILENAME saves to the Downloads folder; a FILENAME with a path, such as ./myvideo, saves there.
  Options may come before or after FILENAME and URL.

Options
  -q, --quality N        Cap video height at N pixels, or width for portrait (default 360; other side pairs at 16:9)
  -w, --width N          Cap video width at N pixels, or height for portrait (default 640; other side pairs at 16:9)
  -b, --best             Uncapped best-quality mp4 (often AV1); may not play in older players
  -c, --cookies BROWSER  Use your login from BROWSER, such as safari, chrome, or firefox, for sites that need one
  -u, --update           Upgrade yt-dlp to the nightly version
  -v, --version          Print dl v2.8.0 and exit
  -h, -?, --help         Show this help and exit

  Without -c, ~/.config/dl/cookies ($XDG_CONFIG_HOME/dl/cookies) picks the browser by site.
  One site per line as HOST BROWSER, such as: reddit.com safari
  A HOST matches itself and its subdomains, such as www.reddit.com.

Examples
  dl myvideo "https://youtube.com/watch?v=..."
  dl -q 480 myvideo "https://youtube.com/watch?v=..."
  dl -b myvideo "https://youtube.com/watch?v=..."
  dl ./myvideo "https://youtube.com/watch?v=..."
  dl -c safari myvideo "https://www.reddit.com/r/..."
  dl -u
```
