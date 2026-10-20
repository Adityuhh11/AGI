import pytest

from agi_memory.chunking.common import ChunkingError
from agi_memory.chunking.video import (
    FRAMES,
    Video,
    video_frame_chunks,
    video_identity,
    video_name_chunk,
)
from agi_memory.types import ChunkKind
from video_fixture import make_video


def test_frames_are_taken_from_the_middle_of_each_part(tmp_path):
    with Video(make_video(tmp_path / "clip.mp4", seconds=4)) as video:
        assert video.duration == pytest.approx(4.0, abs=0.2)
        times = video.frame_times()
        assert len(times) == FRAMES and times[0] == pytest.approx(0.25, abs=0.05)
        first, last = video.frame(times[0], 32), video.frame(times[-1], 32)
    assert first.mode == "RGB" and max(first.size) == 32  # thumbnailed to the side asked for
    assert first.getpixel((10, 10)) != last.getpixel((10, 10))  # the colour changed over time


def test_frames_come_from_the_picture_even_when_the_sound_runs_longer(tmp_path):
    clip = make_video(tmp_path / "talk.mkv", seconds=2, audio_seconds=6)
    with Video(clip) as video:
        assert video.duration == pytest.approx(2.0, abs=0.2)  # the picture's length, not 6 s
        frames = [video.frame(t, 32) for t in video.frame_times()]
        assert len(frames) == FRAMES
        late = video.frame(5.5, 32)  # past the last picture: the last frame, not an error
    assert late.size == frames[-1].size


def test_any_failure_to_open_is_a_chunking_error(tmp_path, monkeypatch):
    def legacy_metadata(*args, **kwargs):
        raise UnicodeDecodeError("utf-8", b"\xff", 0, 1, "invalid start byte")

    monkeypatch.setattr("agi_memory.chunking.video.av.open", legacy_metadata)
    with pytest.raises(ChunkingError, match="unreadable video"):
        Video(tmp_path / "camera.mp4")


def test_a_file_that_is_not_a_video_is_a_chunking_error(tmp_path):
    fake = tmp_path / "fake.mp4"
    fake.write_bytes(b"not a video at all" * 100)
    with pytest.raises(ChunkingError):
        Video(fake)


def test_identity_reads_the_ends_and_changes_with_the_content(tmp_path):
    size = 3 << 20
    one, same, other = tmp_path / "a.mp4", tmp_path / "b.mp4", tmp_path / "c.mp4"
    one.write_bytes(b"a" * size)
    same.write_bytes(b"a" * size)
    other.write_bytes(b"a" * (size - 1) + b"b")  # the same size, the last byte differs
    assert video_identity(one, size) == video_identity(same, size)
    assert video_identity(one, size) != video_identity(other, size)


def test_the_name_chunk_and_frame_chunks_have_distinct_identities():
    name = video_name_chunk("s", "trips/beach.mp4", "abc", 1)
    assert (name.kind, name.text, name.content_hash) == (
        ChunkKind.VIDEO,
        "trips/beach.mp4",
        "name:abc",
    )
    frames = video_frame_chunks("s", "trips/beach.mp4", "abc", [0.5, 1.5], 1)
    assert [f.locator for f in frames] == ['{"t":0.5}', '{"t":1.5}']
    assert all(f.kind is ChunkKind.VIDEO_FRAME and f.text == "" for f in frames)
    assert len({name.id, *(f.id for f in frames)}) == 3
