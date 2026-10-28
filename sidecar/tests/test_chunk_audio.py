import numpy as np
import pytest

from audio_fixture import make_wav, tone
from omni_memory.chunking.audio import (
    STRIDE,
    WINDOW,
    AudioTrack,
    audio_name_chunk,
    audio_window_chunk,
    is_silent,
)
from omni_memory.chunking.common import ChunkingError
from omni_memory.embedding.base import AUDIO_RATE
from omni_memory.types import ChunkKind
from video_fixture import make_video


def windows(path):
    with AudioTrack(path) as track:
        return [(start, len(samples) / AUDIO_RATE) for start, samples in track.windows()]


@pytest.mark.parametrize(
    ("seconds", "want"),
    [
        (0.5, []),  # under a second is not worth a vector
        (10, [(0.0, 10.0)]),
        (30, [(0.0, 30.0)]),  # the second window would hold no new second
        (35, [(0.0, 30.0), (15.0, 20.0)]),
        (40, [(0.0, 30.0), (15.0, 25.0)]),
        (70, [(0.0, 30.0), (15.0, 30.0), (30.0, 30.0), (45.0, 25.0)]),
    ],
)
def test_windows_are_thirty_seconds_every_fifteen_and_cover_every_second(tmp_path, seconds, want):
    assert windows(make_wav(tmp_path / "a.wav", seconds)) == want


def test_a_window_is_never_longer_than_the_model_takes():
    assert WINDOW == 30 * AUDIO_RATE and STRIDE == 15 * AUDIO_RATE


def test_silent_windows_are_skipped_but_the_clock_goes_on(tmp_path):
    clip = make_wav(tmp_path / "a.wav", 60, sounds=((40.0, 60.0),))
    # [0, 30) is silent; [15, 45) and [30, 60) hear something
    assert windows(clip) == [(15.0, 30.0), (30.0, 30.0)]


def test_a_file_of_silence_has_no_windows(tmp_path):
    assert windows(make_wav(tmp_path / "a.wav", 40, sounds=((0.0, 0.0),))) == []


@pytest.mark.parametrize(
    ("window", "silent"),
    [
        (np.zeros(30 * AUDIO_RATE, np.float32), True),
        ((np.random.default_rng(0).standard_normal(30 * AUDIO_RATE) * 0.001).astype("f4"), True),
        ((np.random.default_rng(0).standard_normal(30 * AUDIO_RATE) * 0.006).astype("f4"), False),
        (tone(30, level=0.004), False),  # RMS 0.003 (-51 dBFS): a very quiet recording is still one
        (tone(30), False),
        (np.zeros(0, np.float32), True),
    ],
)
def test_the_gate_drops_digital_silence_and_hiss_but_keeps_room_tone_and_quiet_recordings(
    window, silent
):
    # The model ignores loudness and noise of any level scores like an answer, so the line only
    # removes silence and hiss (docs/eval/2026-10-28-audio-embedding.md, section 4).
    assert is_silent(window) is silent


def test_half_a_second_of_sound_in_a_silent_window_is_enough_to_keep_it():
    window = np.zeros(30 * AUDIO_RATE, np.float32)
    window[10 * AUDIO_RATE : 10 * AUDIO_RATE + AUDIO_RATE // 2] = tone(0.5)
    assert not is_silent(window)


def test_other_rates_and_channel_counts_arrive_as_sixteen_kilohertz_mono(tmp_path):
    clip = make_wav(tmp_path / "cd.wav", 20, rate=44_100, channels=2)
    with AudioTrack(clip) as track:
        (start, samples), *rest = track.windows()
    assert (start, rest) == (0.0, [rest[0]] if rest else [])
    assert samples.dtype == np.float32 and samples.ndim == 1
    assert len(samples) == pytest.approx(30 * AUDIO_RATE * 20 / 30, abs=AUDIO_RATE // 10)


def test_a_video_with_no_sound_has_no_track_and_one_with_sound_has_windows(tmp_path):
    with AudioTrack(make_video(tmp_path / "mute.mkv", seconds=2)) as track:
        assert not track.present and list(track.windows()) == []
    loud = make_video(tmp_path / "talk.mkv", seconds=2, audio_seconds=4, loud=True)
    with AudioTrack(loud) as track:
        assert track.present
        ((start, samples),) = list(track.windows())
    assert (start, len(samples) / AUDIO_RATE) == (0.0, pytest.approx(4.0, abs=0.2))


def test_a_file_that_is_not_audio_is_a_chunking_error(tmp_path):
    fake = tmp_path / "fake.mp3"
    fake.write_bytes(b"not sound at all" * 100)
    with pytest.raises(ChunkingError):
        AudioTrack(fake)


def test_a_failure_to_open_is_a_chunking_error(tmp_path, monkeypatch):
    def refuse(*args, **kwargs):
        raise UnicodeDecodeError("utf-8", b"\xff", 0, 1, "invalid start byte")

    monkeypatch.setattr("omni_memory.chunking.audio.av.open", refuse)
    with pytest.raises(ChunkingError, match="unreadable audio"):
        AudioTrack(tmp_path / "a.mp3")


def test_the_name_chunk_and_window_chunks_have_distinct_identities():
    name = audio_name_chunk("s", "talks/a.mp3", "abc", 1)
    assert (name.kind, name.text, name.content_hash) == (
        ChunkKind.AUDIO,
        "talks/a.mp3",
        "name:abc",
    )
    first = audio_window_chunk("s", "talks/a.mp3", "abc", 0.0, 1)
    second = audio_window_chunk("s", "talks/a.mp3", "abc", 15.0, 1)
    assert (first.kind, first.text) == (ChunkKind.AUDIO_WINDOW, "")
    assert (first.locator, second.locator) == ('{"t":0.0}', '{"t":15.0}')
    assert len({name.id, first.id, second.id}) == 3
    # the same window of the same bytes is the same chunk wherever it is found
    assert first.id == audio_window_chunk("s", "talks/a.mp3", "abc", 0.0, 99).id
    assert first.content_hash != audio_window_chunk("s", "talks/a.mp3", "xyz", 0.0, 1).content_hash
