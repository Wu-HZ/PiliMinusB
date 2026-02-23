import 'package:PiliPlus/plugin/pl_player/controller.dart';
import 'package:PiliPlus/plugin/pl_player/models/play_status.dart';
import 'package:PiliPlus/services/floating_window_manager.dart';
import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:media_kit_video/media_kit_video.dart';

/// A reusable widget for floating video windows
/// Displays a small video player with control buttons
class FloatingVideoWindow extends StatefulWidget {
  final String windowId;
  final PlPlayerController playerController;
  final VoidCallback onTap;
  final VoidCallback onClose;
  final bool isLive;
  final double floatingWidth;
  final double floatingHeight;
  final Offset initialPosition;
  final ValueChanged<Offset>? onPositionChanged;

  const FloatingVideoWindow({
    super.key,
    required this.windowId,
    required this.playerController,
    required this.onTap,
    required this.onClose,
    this.isLive = false,
    this.floatingWidth = 240.0,
    this.floatingHeight = 135.0,
    this.initialPosition = const Offset(20.0, 100.0),
    this.onPositionChanged,
  });

  /// Calculate dimensions based on video aspect ratio
  /// Returns a Size object with width and height
  static Size calculateDimensions(PlPlayerController controller) {
    double? videoHeight =
        controller.videoPlayerController?.state.height?.toDouble();
    double? videoWidth =
        controller.videoPlayerController?.state.width?.toDouble();

    // Calculate aspect ratio based on video orientation
    double aspectRatio = !controller.isVertical ? 9.0 / 16.0 : 16.0 / 9.0;

    if (videoWidth != null && videoHeight != null) {
      if ((videoWidth > videoHeight) ^ controller.isVertical) {
        aspectRatio = videoHeight / videoWidth;
      }
    }

    double floatingWidth = aspectRatio > 1 ? 150.0 : 240.0;
    double floatingHeight = floatingWidth * aspectRatio;

    return Size(floatingWidth, floatingHeight);
  }

  @override
  State<FloatingVideoWindow> createState() => _FloatingVideoWindowState();
}

class _FloatingVideoWindowState extends State<FloatingVideoWindow> {
  late Offset _position;
  final RxBool _showControls = false.obs;
  bool _isHovering = false;

  @override
  void initState() {
    super.initState();
    _position = widget.initialPosition;
  }

  @override
  void dispose() {
    _showControls.close();
    super.dispose();
  }

  void _onEnter(PointerEvent event) {
    _isHovering = true;
    _showControls.value = true;
  }

  void _onExit(PointerEvent event) {
    _isHovering = false;
    _showControls.value = false;
  }

  void _handleTap() {
    widget.onTap();
  }

  @override
  Widget build(BuildContext context) {
    final screenSize = MediaQuery.sizeOf(context);

    return Obx(
      () => floatingWindowManager.isHidden.value
          ? const SizedBox.shrink()
          : Positioned(
              left: _position.dx,
              top: _position.dy,
              child: MouseRegion(
                onEnter: _onEnter,
                onExit: _onExit,
                child: GestureDetector(
                  onPanUpdate: (details) {
                    setState(() {
                      _position = Offset(
                        (_position.dx + details.delta.dx).clamp(
                          0.0,
                          screenSize.width - widget.floatingWidth,
                        ),
                        (_position.dy + details.delta.dy).clamp(
                          0.0,
                          screenSize.height - widget.floatingHeight,
                        ),
                      );
                    });
                    if (!_showControls.value) {
                      _showControls.value = true;
                    }
                  },
                  onPanEnd: (_) {
                    widget.onPositionChanged?.call(_position);
                    if (!_isHovering) {
                      _showControls.value = false;
                    }
                  },
                  onTap: _handleTap,
                  child: Material(
                    elevation: 8,
                    borderRadius: BorderRadius.circular(8),
                    clipBehavior: Clip.antiAlias,
                    child: SizedBox(
                      width: widget.floatingWidth,
                      height: widget.floatingHeight,
                      child: Stack(
                        children: [
                          // Video layer
                          Positioned.fill(
                            child: widget.playerController.videoController !=
                                    null
                                ? Video(
                                    controller: widget
                                        .playerController.videoController!,
                                    controls: NoVideoControls,
                                    pauseUponEnteringBackgroundMode: !widget
                                        .playerController
                                        .continuePlayInBackground
                                        .value,
                                    resumeUponEnteringForegroundMode: true,
                                    subtitleViewConfiguration: widget
                                        .playerController.subtitleConfig.value,
                                    fit: BoxFit.contain,
                                  )
                                : const ColoredBox(
                                    color: Colors.black,
                                    child: Center(
                                      child: Icon(Icons.video_library,
                                          color: Colors.white54),
                                    ),
                                  ),
                          ),
                          // Controls overlay
                          Positioned.fill(
                            child: Obx(
                              () => AnimatedOpacity(
                                opacity: _showControls.value ? 1.0 : 0.0,
                                duration: const Duration(milliseconds: 200),
                                child: IgnorePointer(
                                  ignoring: !_showControls.value,
                                  child: Container(
                                    color: Colors.black26,
                                    child: Stack(
                                      children: [
                                        // Center play/pause button
                                        if (!widget.isLive)
                                          Center(
                                            child: Obx(
                                              () => IconButton(
                                                onPressed: () => widget
                                                    .playerController
                                                    .videoPlayerController
                                                    ?.playOrPause(),
                                                icon: Icon(
                                                  widget
                                                              .playerController
                                                              .playerStatus
                                                              .value ==
                                                          PlayerStatus.playing
                                                      ? Icons.pause_circle_filled
                                                      : Icons.play_circle_filled,
                                                  color: Colors.white,
                                                  size: 48,
                                                ),
                                              ),
                                            ),
                                          ),
                                        // Close button (top-right)
                                        Positioned(
                                          top: 4,
                                          right: 4,
                                          child: IconButton(
                                            onPressed: widget.onClose,
                                            icon: const Icon(
                                              Icons.close,
                                              color: Colors.white,
                                              size: 20,
                                            ),
                                            style: IconButton.styleFrom(
                                              backgroundColor: Colors.black45,
                                              padding: const EdgeInsets.all(4),
                                              minimumSize: const Size(28, 28),
                                            ),
                                          ),
                                        ),
                                      ],
                                    ),
                                  ),
                                ),
                              ),
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),
                ),
              ),
            ),
    );
  }
}
