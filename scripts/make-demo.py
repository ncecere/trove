#!/usr/bin/env python3
"""Create a fake media tree (sparse placeholder files) for trying Trove out.

usage: make-demo.py <target-dir>
"""
import os, sys

root = sys.argv[1]
GB, MB = 1 << 30, 1 << 20

def f(path, size):
    p = os.path.join(root, path)
    os.makedirs(os.path.dirname(p), exist_ok=True)
    with open(p, "wb") as fh:
        fh.truncate(int(size))  # sparse: takes no real disk space

# Movies
f("Movies/Big Buck Bunny (2008)/Big.Buck.Bunny.2008.2160p.WEB-DL.x265.HDR10.AAC-BLENDER.mkv", 6.4 * GB)
f("Movies/Big Buck Bunny (2008)/Big.Buck.Bunny.2008.1080p.BluRay.x264.DTS-BLENDER.mkv", 2.1 * GB)
f("Movies/Big Buck Bunny (2008)/Big.Buck.Bunny.2008.1080p.BluRay.x264.DTS-BLENDER.en.srt", 40_000)
f("Movies/Sintel (2010)/Sintel.2010.1080p.BluRay.x264.AAC.mp4", 1.3 * GB)
f("Movies/Sintel (2010)/Sintel.2010.1080p.BluRay.x264.AAC.en.srt", 22_000)
f("Movies/Sintel (2010)/Sintel.2010.1080p.BluRay.x264.AAC.es.srt", 23_000)
f("Movies/Sintel (2010)/Extras/Making of Sintel.mkv", 310 * MB)
f("Movies/Tears of Steel (2012)/Tears.of.Steel.2012.2160p.WEB-DL.DV.HDR10.HEVC.DDP5.1.mkv", 8.9 * GB)
f("Movies/Night of the Living Dead (1968)/Night.of.the.Living.Dead.1968.REMASTERED.1080p.BluRay.x265.FLAC.mkv", 7.2 * GB)
f("Movies/Nosferatu (1922)/Nosferatu.1922.720p.BluRay.x264.mkv", 3.1 * GB)
f("Movies/Metropolis (1927)/Metropolis.1927.1080p.BluRay.Remux.AVC.DTS-HD.MA.mkv", 28.5 * GB)
f("Movies/The General (1926)/The.General.1926.1080p.BluRay.x264.mkv", 5.5 * GB)
f("Movies/His Girl Friday (1940)/His.Girl.Friday.1940.1080p.BluRay.x264.AAC.mkv", 4.4 * GB)
f("Movies/Charade (1963)/Charade.1963.1080p.BluRay.x265.AAC.mkv", 3.9 * GB)
f("Movies/Elephants Dream (2006)/Elephants.Dream.2006.1080p.WEBRip.x264.mp4", 820 * MB)
f("Movies/The Kid (1921)/The.Kid.1921.1080p.BluRay.x264.mkv", 3.3 * GB)
f("Movies/Plan.9.from.Outer.Space.1957.720p.BluRay.x264-GRP.mkv", 2.2 * GB)

# TV
def show(name, seasons, q="1080p.WEB-DL.x264", size=1.4 * GB, titles=None):
    base = name.split(" (")[0].replace(" ", ".")
    for s, n in seasons.items():
        for e in range(1, n + 1):
            t = (titles or {}).get((s, e))
            tt = "." + t.replace(" ", ".") if t else ""
            folder = "Specials" if s == 0 else f"Season {s:02d}"
            f(f"TV Shows/{name}/{folder}/{base}.S{s:02d}E{e:02d}{tt}.{q}.mkv", size)

show("The Twilight Zone (1959)", {1: 8, 2: 6, 3: 5},
     titles={(1, 1): "Where Is Everybody", (1, 2): "One for the Angels", (1, 3): "Mr Denton on Doomsday"}, q="1080p.BluRay.x265", size=1.1 * GB)
show("The Dick Van Dyke Show (1961)", {1: 6, 2: 4}, q="720p.WEB-DL.x264", size=640 * MB)
show("Doctor Who (1963)", {0: 2, 1: 6, 2: 5}, q="576p.DVD.x264", size=480 * MB)
show("Dragnet (1951)", {1: 5}, q="480p.DVD.x264", size=350 * MB)
show("The Beverly Hillbillies (1962)", {1: 7}, q="720p.HDTV.x264", size=560 * MB)
show("Bonanza (1959)", {1: 6, 2: 6}, q="1080p.WEB-DL.x264", size=1.6 * GB)
f("TV Shows/The Twilight Zone (1959)/Season 01/The.Twilight.Zone.S01E01.Where.Is.Everybody.1080p.BluRay.x265.en.srt", 31_000)

# Anime
for e in range(1, 13):
    f(f"Anime/Astro Boy (1963)/[OldSubs] Astro Boy - {e:02d} [480p][A1B2C3D4].mkv", 230 * MB)
for e in range(1, 9):
    f(f"Anime/Speed Racer (1967)/Season 1/[Retro] Speed Racer - {e:02d} (BD 1080p HEVC FLAC).mkv", 1.2 * GB)
for e in range(1, 7):
    f(f"Anime/Kimba the White Lion (1965)/Kimba the White Lion - S01E{e:02d} [720p].mkv", 410 * MB)
f("Anime/Panda and the Magic Serpent (1958)/Panda.and.the.Magic.Serpent.1958.1080p.BluRay.x264.mkv", 4.1 * GB)

# Books
books = [
    ("Jane Austen", "Pride and Prejudice", ["epub", "mobi", "pdf"]),
    ("Jane Austen", "Emma", ["epub"]),
    ("Mary Shelley", "Frankenstein", ["epub", "pdf"]),
    ("Bram Stoker", "Dracula", ["epub", "azw3"]),
    ("Herman Melville", "Moby Dick", ["epub", "pdf"]),
    ("H. G. Wells", "The War of the Worlds", ["epub"]),
    ("H. G. Wells", "The Time Machine", ["epub", "mobi"]),
    ("Arthur Conan Doyle", "A Study in Scarlet", ["epub"]),
    ("Arthur Conan Doyle", "The Hound of the Baskervilles", ["epub", "pdf"]),
    ("Lewis Carroll", "Alice's Adventures in Wonderland", ["epub", "pdf"]),
    ("Charles Dickens", "Great Expectations", ["epub"]),
    ("Jules Verne", "Twenty Thousand Leagues Under the Sea", ["epub", "mobi"]),
    ("Oscar Wilde", "The Picture of Dorian Gray", ["epub"]),
    ("Robert Louis Stevenson", "Treasure Island", ["epub", "pdf"]),
]
sizes = {"epub": 0.8 * MB, "mobi": 1.1 * MB, "pdf": 4.6 * MB, "azw3": 1.3 * MB}
for a, t, fmts in books:
    for x in fmts:
        f(f"Books/{a}/{t}/{t} - {a}.{x}", sizes[x])

# Audiobooks
for i in range(1, 25):
    f(f"Audiobooks/Mark Twain/Adventures of Huckleberry Finn {{Elijah Wood}}/Huckleberry Finn - Part {i:02d}.mp3", 28 * MB)
for i in range(1, 13):
    f(f"Audiobooks/Arthur Conan Doyle/Sherlock Holmes/1 - The Adventures of Sherlock Holmes {{Stephen Fry}}/{i:02d} - Adventure {i}.mp3", 41 * MB)
f("Audiobooks/Arthur Conan Doyle/Sherlock Holmes/2 - The Memoirs of Sherlock Holmes {Stephen Fry}/The Memoirs of Sherlock Holmes.m4b", 412 * MB)
f("Audiobooks/Homer/The Odyssey (2018) {Claire Danes}/The Odyssey.m4b", 388 * MB)
f("Audiobooks/Homer/The Odyssey (2018) {Claire Danes}/Map and Notes.pdf", 6 * MB)
f("Audiobooks/Jack London/The Call of the Wild/The Call of the Wild.m4b", 96 * MB)
for d in (1, 2):
    for i in range(1, 9):
        f(f"Audiobooks/Charles Dickens/A Christmas Carol {{Tim Curry}}/Disc {d}/Track {i:02d}.mp3", 19 * MB)
print("demo media written to", root)
