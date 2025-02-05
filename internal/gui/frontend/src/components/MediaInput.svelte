<script lang="ts">
    import { OpenDirectoryDialog, OpenVideoDialog, GetVideosInDirectory } from '../../wailsjs/go/gui/App';

    export let mediaSource: MediaSource | null = null;  // Single selected video/directory
    export let previewFiles: MediaSource[] = [];        // Preview only for directories

    interface VideoInfo {
        name: string;
        path: string;
    }
    
    let dragOver = false;
    let isDirectory = false;
    const MAX_VISIBLE_FILES = 4;

    async function handleDirectorySelect() {
        try {
            const dirPath = await OpenDirectoryDialog();
            if (dirPath) {
                isDirectory = true;
                mediaSource = {
                    name: dirPath.split('/').pop() || dirPath,
                    path: dirPath,
                };
                // Get preview of videos in directory
                previewFiles = await GetVideosInDirectory(dirPath);
                console.error('previewFiles are:', previewFiles);
            }
        } catch (error) {
            console.error('Error selecting directory:', error);
        }
    }

    async function handleVideoSelect() {
        try {
            const filePath = await OpenVideoDialog();
            if (filePath) {
                isDirectory = false;
                mediaSource = {
                    name: filePath.split('/').pop() || filePath,
                    path: filePath
                };
                previewFiles = []; // Clear preview as it's not a directory
            }
        } catch (error) {
            console.error('Error selecting video:', error);
        }
    }

    async function handleDrop(e: DragEvent) {
        e.preventDefault();
        dragOver = false;
        
        const files = Array.from(e.dataTransfer?.files || []);
        if (files.length !== 1) return;

        const file = files[0];
        if (file.type.startsWith('video/')) {
            isDirectory = false;
            mediaSource = {
                name: file.name,
                path: file.path
            };
            previewFiles = [];
        } else {
            try {
                isDirectory = true;
                mediaSource = {
                    name: file.name,
                    path: file.path
                };
                const videos = await GetVideosInDirectory(file.path);
                previewFiles = videos.map(v => ({
                    name: v.name,
                    path: v.path,
                    size: v.size
                }));
            } catch (error) {
                console.error('Error handling directory drop:', error);
            }
        }
    }

    function resetSelection() {
        mediaSource = null;
        isDirectory = false;
    }

    function preventDefaults(e: Event) {
        e.preventDefault();
        e.stopPropagation();
    }

    function handleDragEnter(e: DragEvent) {
        preventDefaults(e);
        dragOver = true;
    }

    function handleDragLeave(e: DragEvent) {
        preventDefaults(e);
        const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
        const { clientX, clientY } = e;
        
        if (clientX <= rect.left || clientX >= rect.right || 
            clientY <= rect.top || clientY >= rect.bottom) {
            dragOver = false;
        }
    }

    $: visibleFiles = previewFiles.slice(0, MAX_VISIBLE_FILES);
    $: remainingFiles = previewFiles.length - MAX_VISIBLE_FILES;
</script>


<div class="relative" role="presentation">
    <div
        role="presentation"
        class="relative border-2 border-dashed border-accent/30 rounded-lg p-4 text-center
               transition-all duration-200 ease-out bg-white/5
               hover:border-accent/50 hover:bg-white/10
               {dragOver ? 'border-accent bg-accent/10' : ''}
               {mediaSource ? 'opacity-90' : ''}"
        on:dragenter={handleDragEnter}
        on:dragleave={handleDragLeave}
        on:dragover={preventDefaults}
        on:drop={handleDrop}
    >
        {#if !mediaSource}
            <div class="text-accent/70">
                <span class="material-icons text-2xl mb-1">upload_file</span>
                <p class="text-sm leading-none">
                    Select
                    <button 
                        class="text-accent hover:text-accent-2 underline decoration-dotted
                               leading-none inline hover:-translate-y-0.5
                               transition-all duration-200"
                        on:click={handleDirectorySelect}
                    >a directory</button>
                    or
                    <button 
                        class="text-accent hover:text-accent-2 underline decoration-dotted
                               leading-none inline hover:-translate-y-0.5
                               transition-all duration-200"
                        on:click={handleVideoSelect}
                    >a video</button>
                    to process
                </p>
            </div>
        {:else}
            <div class="text-left">
                <div class="space-y-2">
                    {#if isDirectory}
                        <div class="flex items-center justify-between gap-1 p-1.5 bg-accent/10 rounded text-sm">
                            <div class="flex items-center gap-1 min-w-0">
                                <span class="material-icons text-accent text-sm flex-shrink-0">folder</span>
                                <span class="truncate">{mediaSource.path}</span>
                            </div>
                            <button 
                                class="flex-shrink-0 flex-grow-0 text-red-400/70 hover:text-red-400 
                                       transition-colors duration-200 
                                       aspect-square w-1 min-w-[8px] max-w-[8px]
                                       inline-flex items-center justify-center"
                                on:click={resetSelection}
                            >
                                <span class="material-icons text-[14px]">close</span>
                            </button>
                        </div>
                        <div class="pl-8 space-y-1">
                            {#each visibleFiles as file, i}
                                <div class="flex text-sm">
                                    <!-- Tree connection lines container -->
                                    <div class="relative w-6 flex-shrink-0">
                                        {#if i !== visibleFiles.length - 1 || remainingFiles > 0}
                                            <!-- Regular item: vertical + horizontal lines -->
                                            <div class="absolute left-0 top-[-4px] bottom-[-4px] tree-line border-l"></div>
                                            <div class="absolute left-0 top-[0.7em] w-full tree-line border-t"></div>
                                        {:else}
                                            <!-- Last item: L-shaped connector with 1px overlap -->
                                            <div class="absolute left-0 top-[-4px] h-[calc(0.7em_+_3px)] tree-line border-l"></div>
                                            <div class="absolute left-0 top-[0.7em] w-full tree-line border-t"></div>
                                        {/if}
                                    </div>
                                    <!-- Content -->
                                    <div class="flex items-center gap-2 min-w-0">
                                        <span class="material-icons text-accent text-sm flex-shrink-0">movie</span>
                                        <span class="truncate">{file.name}</span>
                                    </div>
                                </div>
                            {/each}
                            {#if remainingFiles > 0}
                                <div class="flex text-sm text-white/40">
                                    <!-- Tree connection lines container -->
                                    <div class="relative w-6 flex-shrink-0">
                                        <!-- Last item: L-shaped connector with 1px overlap -->
                                        <div class="absolute left-0 top-[-4px] h-[0.1em] tree-line border-l"></div>
                                        <div class="absolute left-0 top-[0.1em] w-full tree-line border-t"></div>
                                    </div>
                                    <!-- Content -->
                                    <div class="flex items-center gap-2">
                                        <span class="material-icons text-sm flex-shrink-0">more_horiz</span>
                                        <span>+{remainingFiles} more video{remainingFiles === 1 ? '' : 's'}</span>
                                    </div>
                                </div>
                            {/if}
                        </div>
                    {:else}
                        <div class="flex items-center justify-between gap-1 p-1.5 bg-accent/10 rounded text-sm">
                            <div class="flex items-center gap-1 min-w-0">
                                <span class="material-icons text-accent text-sm flex-shrink-0">movie</span>
                                <span class="truncate">{mediaSource.path}</span>
                            </div>
                            <button 
                                class="flex-shrink-0 flex-grow-0 text-red-400/70 hover:text-red-400 
                                       transition-colors duration-200 
                                       aspect-square w-1 min-w-[8px] max-w-[8px]
                                       inline-flex items-center justify-center"
                                on:click={resetSelection}
                            >
                                <span class="material-icons text-[14px]">close</span>
                            </button>
                        </div>
                    {/if}
                </div>
            </div>
        {/if}
    </div>
</div>

<style>
    .tree-line {
        border-width: 2px;
        border-color: rgb(255 255 255);
    }
</style>