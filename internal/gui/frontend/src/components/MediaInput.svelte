<script lang="ts">
    import { OpenDirectoryDialog, OpenVideoDialog, GetVideosInDirectory } from '../api/services/media';
    import { logger } from '../lib/logger';
    import { isBrowserMode } from '../lib/runtime/bridge';

    export let mediaSource: MediaSource | null = null;  // Single selected video/directory
    export let previewFiles: MediaSource[] = [];        // Preview only for directories
    export let droppedFilePath: string | null = null;   // File path from global drag and drop

    interface VideoInfo {
        name: string;
        path: string;
    }
    
    let dragOver = false;
    let isDirectory = false;
    const VISIBLE_HEIGHT_VIDEOS = 4; // Number of videos visible without scrolling

    async function handleDirectorySelect() {
        logger.trace('MediaInput', 'Directory selection initiated');
        try {
            const dirPath = await OpenDirectoryDialog();
            if (dirPath) {
                isDirectory = true;
                mediaSource = {
                    name: dirPath.split('/').pop() || dirPath,
                    path: dirPath,
                };
                logger.info('MediaInput', 'Directory selected', { path: dirPath });
                
                // Get preview of videos in directory
                const videos = await GetVideosInDirectory(dirPath);
                previewFiles = videos.map(v => ({
                    name: v.Name,
                    path: v.Path
                }));
                logger.debug('MediaInput', 'Videos found in directory', { 
                    path: dirPath, 
                    videoCount: previewFiles.length 
                });
            } else {
                logger.trace('MediaInput', 'Directory selection cancelled');
            }
        } catch (error) {
            logger.error('MediaInput', 'Error selecting directory', { error });
        }
    }

    async function handleVideoSelect() {
        logger.trace('MediaInput', 'Video selection initiated');
        try {
            const filePath = await OpenVideoDialog();
            if (filePath) {
                isDirectory = false;
                mediaSource = {
                    name: filePath.split('/').pop() || filePath,
                    path: filePath
                };
                previewFiles = []; // Clear preview as it's not a directory
                logger.info('MediaInput', 'Video file selected', { 
                    path: filePath, 
                    name: mediaSource.name 
                });
            } else {
                logger.trace('MediaInput', 'Video selection cancelled');
            }
        } catch (error) {
            logger.error('MediaInput', 'Error selecting video', { error });
        }
    }

    // React to dropped file from global drag and drop
    $: if (droppedFilePath) {
        handleDroppedFile(droppedFilePath);
    }
    
    async function handleDroppedFile(filePath: string) {
        logger.debug('MediaInput', 'Processing dropped file', { filePath });
        
        const fileName = filePath.split('/').pop() || filePath;
        
        // Check if it's a video file by extension first
        const videoExts = ['.mp4', '.mkv', '.avi', '.mov', '.wmv', '.flv', '.webm', '.m4v'];
        const ext = fileName.substring(fileName.lastIndexOf('.')).toLowerCase();
        
        if (videoExts.includes(ext)) {
            // It's a video file
            isDirectory = false;
            mediaSource = {
                name: fileName,
                path: filePath
            };
            previewFiles = [];
            logger.info('MediaInput', 'Video file dropped', { 
                path: filePath, 
                name: fileName 
            });
        } else {
            // Not a video file, check if it's a directory
            try {
                const videos = await GetVideosInDirectory(filePath);
                // It's a directory
                isDirectory = true;
                mediaSource = {
                    name: fileName,
                    path: filePath
                };
                previewFiles = videos.map(v => ({
                    name: v.Name,
                    path: v.Path
                }));
                logger.info('MediaInput', 'Directory dropped', { 
                    path: filePath, 
                    videoCount: previewFiles.length 
                });
            } catch (error) {
                logger.warn('MediaInput', 'Dropped item is neither a video file nor an accessible directory', { 
                    path: filePath,
                    error 
                });
            }
        }
        
        // Reset the dropped file path after processing
        droppedFilePath = null;
    }

    function resetSelection() {
        logger.info('MediaInput', 'Media selection reset');
        mediaSource = null;
        isDirectory = false;
        previewFiles = [];
    }

    // Visual feedback for drag over - simplified since Wails handles the actual drop
    function handleDragEnter(e: DragEvent) {
        e.preventDefault();
        if (!isBrowserMode()) {
            dragOver = true;
            logger.trace('MediaInput', 'Drag entered drop zone');
        }
    }

    function handleDragLeave(e: DragEvent) {
        e.preventDefault();
        if (!isBrowserMode()) {
            dragOver = false;
            logger.trace('MediaInput', 'Drag left drop zone');
        }
    }
    
    function handleDragOver(e: DragEvent) {
        e.preventDefault(); // Still needed to allow drop
    }
</script>

<div class="relative" role="presentation">
    <div
        role="presentation"
        class="relative border-2 border-dashed border-primary/30 rounded-lg p-4 text-center
               transition-all duration-200 ease-out bg-ui-element
               hover:border-primary/50 hover:bg-ui-element-hover
               {dragOver ? 'border-primary bg-primary/10 scale-[1.01]' : ''}
               {mediaSource ? 'opacity-95' : ''}"
        on:dragenter={handleDragEnter}
        on:dragleave={handleDragLeave}
        on:dragover={handleDragOver}
    >
        {#if !mediaSource}
            <!-- No media selected state -->
            <div class="text-primary/80 flex flex-col items-center justify-center gap-2 py-1">
                <div class="flex items-center flex-wrap justify-center gap-1 text-sm text-gray-300">
                    <!-- First item will wrap separately -->
                    <span class="w-full text-center mb-1">
                        {#if !isBrowserMode()}
                            Drag &amp; drop here or select
                        {:else}
                            Select
                        {/if}
                    </span>
                    
                    <!-- Group buttons together to keep them on same line -->
                    <div class="flex items-center flex-nowrap">
                        <button 
                            class="px-2 py-0.5 mx-1 text-sm font-medium rounded-md bg-primary/20 text-primary
                                   hover:bg-primary/30 hover:shadow-md hover:-translate-y-0.5
                                   transition-all duration-200 inline-flex items-center gap-1"
                            on:click={handleDirectorySelect}
                        >
                            <span class="material-icons text-sm">folder</span>
                            <span>directory</span>
                        </button>
                        <span>/</span>
                        <button 
                            class="px-2 py-0.5 mx-1 text-sm font-medium rounded-md bg-primary/20 text-primary
                                   hover:bg-primary/30 hover:shadow-md hover:-translate-y-0.5
                                   transition-all duration-200 inline-flex items-center gap-1"
                            on:click={handleVideoSelect}
                        >
                            <span class="material-icons text-sm">movie</span>
                            <span>video</span>
                        </button>
                    </div>
                </div>
            </div>
        {:else}
            <!-- Media selected state -->
            <div class="text-left">
                <div class="space-y-2">
                    <!-- Selected source display -->
                    <div class="flex items-center justify-between gap-1 p-2 bg-primary/10 rounded text-sm border border-primary/20 hover:border-primary/40 transition-colors duration-200">
                        <div class="flex items-center gap-2 min-w-0">
                            <span class="material-icons text-primary flex-shrink-0">
                                {isDirectory ? 'folder' : 'movie'}
                            </span>
                            <span class="truncate font-medium">{mediaSource.path}</span>
                        </div>
                        <button 
                            class="flex-shrink-0 text-red-400/70 hover:text-red-400 hover:bg-white/10
                                   p-1 rounded-md transition-all duration-200
                                   inline-flex items-center justify-center"
                            on:click={resetSelection}
                            title="Remove selection"
                        >
                            <span class="material-icons text-[16px]">close</span>
                        </button>
                    </div>

                    {#if isDirectory && previewFiles.length > 0}
                        <!-- Directory content preview with tree structure -->
                        <div class="bg-ui-element p-2 rounded-md">
                            <!-- Header with total count -->
                            <div class="flex justify-between items-center mb-1 text-xs text-gray-300">
                                <span class="font-medium">Directory contents:</span>
                                <span>{previewFiles.length} video{previewFiles.length === 1 ? '' : 's'} in total</span>
                            </div>
                            
                            <!-- Optimized scrollable file list with tree structure and windowing -->
                            <div class="pl-2 space-y-0.5 max-h-[140px] overflow-y-auto pr-1 custom-scrollbar">
                                {#if previewFiles.length <= 10}
                                    <!-- For small lists, render everything directly -->
                                    {#each previewFiles as file, i (file.path)}
                                        <div class="flex text-xs" style="contain: content;">
                                            <!-- Tree connection lines container -->
                                            <div class="relative w-6 flex-shrink-0">
                                                {#if i !== previewFiles.length - 1}
                                                    <!-- Regular item: vertical + horizontal lines -->
                                                    <div class="absolute left-0 top-[-4px] bottom-[-4px] tree-line border-l"></div>
                                                    <div class="absolute left-0 top-[0.7em] w-full tree-line border-t"></div>
                                                {:else}
                                                    <!-- Last item: L-shaped connector -->
                                                    <div class="absolute left-0 top-[-4px] h-[calc(0.7em_+_3px)] tree-line border-l"></div>
                                                    <div class="absolute left-0 top-[0.7em] w-full tree-line border-t"></div>
                                                {/if}
                                            </div>
                                            
                                            <!-- File content -->
                                            <div class="flex items-center gap-2 min-w-0 py-0.5 hover:bg-white/5 rounded pl-1 pr-2 transition-colors duration-150">
                                                <span class="material-icons text-primary text-sm flex-shrink-0">movie</span>
                                                <span class="truncate text-gray-300 hover:text-white">{file.name}</span>
                                            </div>
                                        </div>
                                    {/each}
                                {:else}
                                    <!-- For larger lists, show top few files and a summary -->
                                    {#each previewFiles.slice(0, 8) as file, i (file.path)}
                                        <div class="flex text-xs" style="contain: content;">
                                            <!-- Tree connection lines container -->
                                            <div class="relative w-6 flex-shrink-0">
                                                <div class="absolute left-0 top-[-4px] bottom-[-4px] tree-line border-l"></div>
                                                <div class="absolute left-0 top-[0.7em] w-full tree-line border-t"></div>
                                            </div>
                                            
                                            <!-- File content -->
                                            <div class="flex items-center gap-2 min-w-0 py-0.5 hover:bg-white/5 rounded pl-1 pr-2 transition-colors duration-150">
                                                <span class="material-icons text-primary text-sm flex-shrink-0">movie</span>
                                                <span class="truncate text-gray-300 hover:text-white">{file.name}</span>
                                            </div>
                                        </div>
                                    {/each}
                                    
                                    <!-- Summary line for remaining files -->
                                    <div class="flex text-xs" style="contain: content;">
                                        <div class="relative w-6 flex-shrink-0">
                                            <div class="absolute left-0 top-[-4px] h-[calc(0.7em_+_3px)] tree-line border-l"></div>
                                            <div class="absolute left-0 top-[0.7em] w-full tree-line border-t"></div>
                                        </div>
                                        
                                        <div class="flex items-center gap-2 min-w-0 py-0.5 bg-white/5 rounded pl-1 pr-2">
                                            <span class="material-icons text-primary text-sm flex-shrink-0">more_horiz</span>
                                            <span class="text-gray-300">and {previewFiles.length - 8} more files...</span>
                                        </div>
                                    </div>
                                {/if}
                            </div>
                        </div>
                    {/if}
                </div>
            </div>
        {/if}
    </div>
</div>

<style>
    /* Custom hover effects for the buttons */
    button:active {
        transform: translateY(0) !important;
        transition-duration: 50ms;
    }
    
    /* Tree line styling */
    .tree-line {
        border-color: rgba(255, 255, 255, 0.2);
        border-width: 1px;
    }
    
    /* Custom scrollbar */
    .custom-scrollbar::-webkit-scrollbar {
        width: 6px;
    }
    
    .custom-scrollbar::-webkit-scrollbar-track {
        background: rgba(255, 255, 255, 0.05);
        border-radius: 3px;
    }
    
    .custom-scrollbar::-webkit-scrollbar-thumb {
        background: rgba(255, 255, 255, 0.2);
        border-radius: 3px;
    }
    
    .custom-scrollbar::-webkit-scrollbar-thumb:hover {
        background: rgba(255, 255, 255, 0.3);
    }
</style>