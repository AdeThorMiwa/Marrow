#!/usr/bin/env python3
"""Bridges Go to instaloader's Python library API directly, instead of its
CLI. The CLI's --count/-c flag only applies to hashtag/location/feed/saved
targets, NOT a plain profile target (confirmed against a real profile: it
walks the ENTIRE post history regardless of --count) — no good for a "poll
for the N most recent posts" adapter. Profile.get_posts() is just a
generator, so this script iterates it directly and stops after `limit`
itself, which the CLI has no way to do.

Auth: reads a raw browser `Cookie:` header string from the
INSTALOADER_COOKIES env var (never a CLI arg, to keep it out of `ps`) and
loads it as an instaloader session — same UX as the Twitter adapter's
pasted-cookie flow, not instaloader's own --login flow (which can hit
2FA/challenge prompts).

Usage:
  instaloader_client.py resolve <username>
  instaloader_client.py posts <username> <limit>
  instaloader_client.py post <shortcode>

Output: resolve and post each print one JSON object; posts prints one JSON
object per line (newline-delimited), each a minimal projection of a Post —
only the fields the Go adapter actually reads, not instaloader's full raw
node.

`post` re-fetches a single already-known post by shortcode — used to get a
fresh signed video_url at playback time, since Instagram's CDN URLs expire
within hours of being issued and the one captured at ingest is long gone
by the time a user actually watches (confirmed live: a 403 on the expired
URL). Same media_url shape as `posts`, so the Go side reuses one struct.
"""
import json
import os
import sys

import instaloader


def parse_cookie_header(raw: str) -> dict:
    cookies = {}
    for part in raw.split(";"):
        part = part.strip()
        if not part or "=" not in part:
            continue
        key, _, value = part.partition("=")
        cookies[key.strip()] = value.strip()
    return cookies


def build_context() -> instaloader.InstaloaderContext:
    raw_cookies = os.environ.get("INSTALOADER_COOKIES", "")
    username = os.environ.get("INSTALOADER_USERNAME", "")
    if not raw_cookies or not username:
        print("INSTALOADER_COOKIES and INSTALOADER_USERNAME must both be set", file=sys.stderr)
        sys.exit(1)

    loader = instaloader.Instaloader(
        download_pictures=False, download_videos=False, download_video_thumbnails=False,
        download_geotags=False, download_comments=False, save_metadata=False,
        compress_json=False, quiet=True,
    )
    loader.context.load_session(username, parse_cookie_header(raw_cookies))
    return loader.context


def post_media(post: "instaloader.Post"):
    """Returns one entry per media item — a carousel (GraphSidecar) post has
    several, everything else exactly one."""
    if post.typename == "GraphSidecar":
        return [
            {"is_video": node.is_video, "display_url": node.display_url, "video_url": node.video_url}
            for node in post.get_sidecar_nodes()
        ]
    return [{"is_video": post.is_video, "display_url": post.url, "video_url": post.video_url}]


def cmd_resolve(context: "instaloader.InstaloaderContext", username: str):
    profile = instaloader.Profile.from_username(context, username)
    print(json.dumps({
        "userid": profile.userid,
        "username": profile.username,
        "full_name": profile.full_name,
        "profile_pic_url": profile.profile_pic_url,
    }))


def cmd_posts(context: "instaloader.InstaloaderContext", username: str, limit: int):
    profile = instaloader.Profile.from_username(context, username)
    count = 0
    for post in profile.get_posts():
        if count >= limit:
            break
        count += 1
        print(json.dumps({
            "shortcode": post.shortcode,
            "url": f"https://www.instagram.com/p/{post.shortcode}/",
            "date": post.date_utc.isoformat() + "Z",
            "caption": post.caption or "",
            "media": post_media(post),
        }))


def cmd_post(context: "instaloader.InstaloaderContext", shortcode: str):
    post = instaloader.Post.from_shortcode(context, shortcode)
    print(json.dumps({
        "shortcode": post.shortcode,
        "url": f"https://www.instagram.com/p/{post.shortcode}/",
        "date": post.date_utc.isoformat() + "Z",
        "caption": post.caption or "",
        "media": post_media(post),
    }))


def main():
    if len(sys.argv) < 3:
        print("usage: instaloader_client.py {resolve|posts|post} <username|shortcode> [limit]", file=sys.stderr)
        sys.exit(2)

    mode, target = sys.argv[1], sys.argv[2]
    context = build_context()

    try:
        if mode == "resolve":
            cmd_resolve(context, target)
        elif mode == "posts":
            limit = int(sys.argv[3]) if len(sys.argv) > 3 else 10
            cmd_posts(context, target, limit)
        elif mode == "post":
            cmd_post(context, target)
        else:
            print(f"unknown mode: {mode}", file=sys.stderr)
            sys.exit(2)
    except instaloader.exceptions.InstaloaderException as e:
        print(f"instaloader error: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
