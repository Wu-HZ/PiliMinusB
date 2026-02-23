import 'dart:async';
import 'dart:io';

import 'package:PiliPlus/utils/platform_utils.dart';
import 'package:PiliPlus/utils/storage_pref.dart';
import 'package:PiliPlus/utils/utils.dart';
import 'package:auto_orientation/auto_orientation.dart';
import 'package:flutter/services.dart';
import 'package:flutter/widgets.dart';
import 'package:window_manager/window_manager.dart';

bool _isDesktopFullScreen = false;
Rect? _windowBoundsBeforeFullscreen;

@pragma('vm:notify-debugger-on-exception')
Future<void> enterDesktopFullscreen({bool inAppFullScreen = false}) async {
  if (!inAppFullScreen && !_isDesktopFullScreen) {
    _isDesktopFullScreen = true;
    try {
      // Save window bounds before entering fullscreen
      _windowBoundsBeforeFullscreen = await windowManager.getBounds();
      await const MethodChannel(
        'com.alexmercerind/media_kit_video',
      ).invokeMethod('Utils.EnterNativeFullscreen');
    } catch (_) {}
  }
}

@pragma('vm:notify-debugger-on-exception')
Future<void> exitDesktopFullscreen() async {
  if (_isDesktopFullScreen) {
    _isDesktopFullScreen = false;
    try {
      await const MethodChannel(
        'com.alexmercerind/media_kit_video',
      ).invokeMethod('Utils.ExitNativeFullscreen');
      // Restore window bounds to the exact position before fullscreen
      if (_windowBoundsBeforeFullscreen != null) {
        // Small delay to let the native fullscreen exit complete
        await Future.delayed(const Duration(milliseconds: 50));
        await windowManager.setBounds(_windowBoundsBeforeFullscreen!);
        _windowBoundsBeforeFullscreen = null;
      }
    } catch (_) {}
  }
}

//横屏
@pragma('vm:notify-debugger-on-exception')
Future<void> landscape() async {
  try {
    await AutoOrientation.landscapeAutoMode(forceSensor: true);
  } catch (_) {}
}

//竖屏
Future<void> verticalScreenForTwoSeconds() async {
  await SystemChrome.setPreferredOrientations([DeviceOrientation.portraitUp]);
  await autoScreen();
}

//全向
bool allowRotateScreen = Pref.allowRotateScreen;
Future<void> autoScreen() async {
  if (PlatformUtils.isMobile && allowRotateScreen) {
    await SystemChrome.setPreferredOrientations([
      DeviceOrientation.portraitUp,
      // DeviceOrientation.portraitDown,
      DeviceOrientation.landscapeLeft,
      DeviceOrientation.landscapeRight,
    ]);
  }
}

Future<void> fullAutoModeForceSensor() {
  return AutoOrientation.fullAutoMode(forceSensor: true);
}

bool _showStatusBar = true;
Future<void> hideStatusBar() async {
  if (!_showStatusBar) {
    return;
  }
  _showStatusBar = false;
  await SystemChrome.setEnabledSystemUIMode(SystemUiMode.immersiveSticky);
}

//退出全屏显示
Future<void> showStatusBar() async {
  if (_showStatusBar) {
    return;
  }
  _showStatusBar = true;
  SystemUiMode mode;
  if (Platform.isAndroid && (await Utils.sdkInt < 29)) {
    mode = SystemUiMode.manual;
  } else {
    mode = SystemUiMode.edgeToEdge;
  }
  await SystemChrome.setEnabledSystemUIMode(
    mode,
    overlays: SystemUiOverlay.values,
  );
}
