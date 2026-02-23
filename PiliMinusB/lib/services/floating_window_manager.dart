import 'package:PiliPlus/models/common/video/video_type.dart';
import 'package:PiliPlus/plugin/pl_player/controller.dart';
import 'package:flutter/material.dart';
import 'package:get/get.dart';

/// State for a single floating window
class FloatingWindowState {
  final String windowId;
  final String bvid;
  final int cid;
  final String heroTag;
  final PlPlayerController playerController;
  final OverlayEntry overlayEntry;
  final DateTime createdAt;
  final Duration savedPosition;
  final VideoType videoType;

  // Video metadata for display/resumption
  final String? title;
  final String? coverUrl;
  final int? aid;
  final int? epId;
  final int? seasonId;

  // Window position state
  Offset position;
  final Size size;

  FloatingWindowState({
    required this.windowId,
    required this.bvid,
    required this.cid,
    required this.heroTag,
    required this.playerController,
    required this.overlayEntry,
    required this.videoType,
    required this.savedPosition,
    required this.position,
    required this.size,
    this.title,
    this.coverUrl,
    this.aid,
    this.epId,
    this.seasonId,
  }) : createdAt = DateTime.now();
}

/// Manages multiple floating windows and their associated player controllers
class FloatingWindowManager {
  static final FloatingWindowManager _instance = FloatingWindowManager._();
  static FloatingWindowManager get instance => _instance;
  FloatingWindowManager._();

  /// Map of windowId -> FloatingWindowState
  final Map<String, FloatingWindowState> _windows = {};

  /// Maximum number of floating windows allowed (for resource management)
  int _maxWindows = 3;

  /// Window ID that currently has audio focus
  String? _audioFocusWindowId;

  /// Whether floating windows should be hidden (e.g., during fullscreen)
  final RxBool isHidden = false.obs;

  /// Generate unique window ID
  String generateWindowId() =>
      'floating_${DateTime.now().millisecondsSinceEpoch}';

  /// Get all active window IDs
  List<String> get activeWindowIds => _windows.keys.toList();

  /// Get all active windows
  List<FloatingWindowState> get activeWindows => _windows.values.toList();

  /// Get window count
  int get windowCount => _windows.length;

  /// Check if can create new window
  bool get canCreateWindow => _windows.length < _maxWindows;

  /// Get max windows setting
  int get maxWindows => _maxWindows;

  /// Set max windows (1-5)
  set maxWindows(int value) {
    _maxWindows = value.clamp(1, 5);
    // Close excess windows if needed
    while (_windows.length > _maxWindows) {
      closeOldestWindow();
    }
  }

  /// Create new floating window
  /// Returns windowId on success, null if max reached and autoClose is false
  String? createWindow(
    FloatingWindowState state, {
    bool autoCloseOldest = true,
  }) {
    if (_windows.length >= _maxWindows) {
      if (autoCloseOldest) {
        closeOldestWindow();
      } else {
        return null;
      }
    }

    _windows[state.windowId] = state;

    // Track audio focus but don't mute other windows
    // Let all windows play audio - user can close ones they don't want
    _audioFocusWindowId = state.windowId;

    return state.windowId;
  }

  /// Close specific window by ID
  void closeWindow(String windowId) {
    final window = _windows.remove(windowId);
    if (window == null) return;

    // Remove overlay entry
    window.overlayEntry.remove();

    // Clear audio focus if this window had it
    if (_audioFocusWindowId == windowId) {
      _audioFocusWindowId = null;
      // Update focus tracking to most recent window (but don't mute others)
      if (_windows.isNotEmpty) {
        final mostRecent = _windows.values
            .reduce((a, b) => a.createdAt.isAfter(b.createdAt) ? a : b);
        _audioFocusWindowId = mostRecent.windowId;
      }
    }

    // Note: The player disposal is handled by the caller since they may want
    // to transfer the player back to main view
  }

  /// Close window and dispose its player
  void closeWindowAndDispose(String windowId) {
    final window = _windows[windowId];
    if (window == null) return;

    closeWindow(windowId);

    // Dispose the player
    window.playerController.dispose();
  }

  /// Close all windows
  void closeAllWindows({bool disposeControllers = true}) {
    final windowIds = List<String>.from(_windows.keys);
    for (final windowId in windowIds) {
      if (disposeControllers) {
        closeWindowAndDispose(windowId);
      } else {
        closeWindow(windowId);
      }
    }
    _audioFocusWindowId = null;
  }

  /// Get window state by ID
  FloatingWindowState? getWindow(String windowId) => _windows[windowId];

  /// Get window by index (ordered by creation time)
  FloatingWindowState? getWindowAt(int index) {
    if (index < 0 || index >= _windows.length) return null;
    final sorted = _windows.values.toList()
      ..sort((a, b) => a.createdAt.compareTo(b.createdAt));
    return sorted[index];
  }

  /// Close oldest window
  void closeOldestWindow({bool disposeController = true}) {
    if (_windows.isEmpty) return;

    final oldest = _windows.values
        .reduce((a, b) => a.createdAt.isBefore(b.createdAt) ? a : b);

    if (disposeController) {
      closeWindowAndDispose(oldest.windowId);
    } else {
      closeWindow(oldest.windowId);
    }
  }

  /// Find window by bvid (video ID)
  FloatingWindowState? findWindowByBvid(String bvid) {
    try {
      return _windows.values.firstWhere((w) => w.bvid == bvid);
    } catch (_) {
      return null;
    }
  }

  /// Find window by heroTag
  FloatingWindowState? findWindowByHeroTag(String heroTag) {
    try {
      return _windows.values.firstWhere((w) => w.heroTag == heroTag);
    } catch (_) {
      return null;
    }
  }

  /// Check if a video is in a floating window
  bool hasWindowForVideo(String bvid) => findWindowByBvid(bvid) != null;

  /// Check if a window exists by ID
  bool hasWindow(String windowId) => _windows.containsKey(windowId);

  /// Get window ID that has audio focus
  String? get audioFocusWindowId => _audioFocusWindowId;

  /// Set which window has audio focus (unmuted)
  void setAudioFocus(String windowId) {
    if (!_windows.containsKey(windowId)) return;
    if (_audioFocusWindowId == windowId) return;

    // Mute previous window
    if (_audioFocusWindowId != null) {
      final prev = _windows[_audioFocusWindowId!];
      prev?.playerController.setMute(true);
    }

    // Unmute new window
    final current = _windows[windowId];
    current?.playerController.setMute(false);
    _audioFocusWindowId = windowId;
  }

  /// Called when user taps on a floating window
  void onWindowTapped(String windowId) {
    // Just track which window was tapped, don't mute others
    if (_windows.containsKey(windowId)) {
      _audioFocusWindowId = windowId;
    }
  }

  /// Get the staggered position for a new window (to avoid overlapping)
  double getStaggeredTopPosition() {
    const baseTop = 100.0;
    const staggerOffset = 60.0;
    return baseTop + (_windows.length * staggerOffset);
  }

  /// Transfer a floating window back to main view
  /// Returns the window state and removes it from management
  /// (but does NOT dispose the player - caller should reuse it)
  FloatingWindowState? transferToMainView(String windowId) {
    final window = _windows.remove(windowId);
    if (window == null) return null;

    // Remove the overlay entry
    window.overlayEntry.remove();

    // Update audio focus tracking
    if (_audioFocusWindowId == windowId) {
      _audioFocusWindowId = null;
      if (_windows.isNotEmpty) {
        final mostRecent = _windows.values
            .reduce((a, b) => a.createdAt.isAfter(b.createdAt) ? a : b);
        _audioFocusWindowId = mostRecent.windowId;
      }
    }

    return window;
  }

  /// Transfer a floating window by bvid
  FloatingWindowState? transferToMainViewByBvid(String bvid) {
    final window = findWindowByBvid(bvid);
    if (window == null) return null;
    return transferToMainView(window.windowId);
  }

  /// Update window position
  void updateWindowPosition(String windowId, Offset position) {
    final window = _windows[windowId];
    if (window != null) {
      window.position = position;
    }
  }
}

/// Global instance for easy access
final floatingWindowManager = FloatingWindowManager.instance;
