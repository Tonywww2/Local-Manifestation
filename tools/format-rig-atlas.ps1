# Windows PowerShell 7 image-formatting helper. Does not generate or repaint artwork.
# Finds four row bands and six detached parts per row using alpha > 16, then
# crops and proportionally packs each part into a 256px cell with 16px padding.
# If columns have staggered vertical placement, detects columns first instead.
# Preserves source files, fails on ambiguous layouts, emits reproducible crop
# rectangles and requires every final cell border to remain fully transparent.
# Example: ./tools/format-rig-atlas.ps1 -InputPath docs/art-v1/sources/animation/bagbound.png -OutputPath content/base/assets/art-v1/animation/bagbound.png
param([Parameter(Mandatory=$true)][string]$InputPath,[Parameter(Mandatory=$true)][string]$OutputPath)
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Drawing
$taskSource = @'
using System;
using System.Collections.Generic;
using System.Drawing;
using System.Drawing.Imaging;
using System.Drawing.Drawing2D;
using System.Runtime.InteropServices;
public static class ArtAtlasFormatter {
  static List<int[]> Bands(int[] counts, int minimum) {
    var found = new List<int[]>();
    int start=-1, last=-1;
    for(int i=0;i<counts.Length;i++) {
      if(counts[i]>=minimum) { if(start<0)start=i;last=i; }
      else if(start>=0 && i-last>6) { if(last-start+1>=8)found.Add(new int[]{start,last+1});start=-1; }
    }
    if(start>=0 && last-start+1>=8)found.Add(new int[]{start,last+1});
    return found;
  }
  public static object Format(string input, string output) {
    using(var source=new Bitmap(input))
    using(var src=source.Clone(new Rectangle(0,0,source.Width,source.Height),PixelFormat.Format32bppArgb))
    using(var dst=new Bitmap(1536,1024,PixelFormat.Format32bppArgb)) {
      var bits=src.LockBits(new Rectangle(0,0,src.Width,src.Height),ImageLockMode.ReadOnly,PixelFormat.Format32bppArgb);
      int stride=Math.Abs(bits.Stride);var rgba=new byte[stride*src.Height];Marshal.Copy(bits.Scan0,rgba,0,rgba.Length);src.UnlockBits(bits);
      var yc=new int[src.Height];int zero=0,maxAlpha=0;
      for(int y=0;y<src.Height;y++)for(int x=0;x<src.Width;x++){int a=rgba[y*stride+x*4+3];if(a>16)yc[y]++;if(a==0)zero++;maxAlpha=Math.Max(maxAlpha,a);}
      if(zero==0)throw new Exception("No genuine transparent background: "+input);
      var rows=Bands(yc,8);List<int[]> globalColumns=null;List<int[]>[] columnRows=null;
      if(rows.Count!=4) {
        var xc=new int[src.Width];for(int y=0;y<src.Height;y++)for(int x=0;x<src.Width;x++)if(rgba[y*stride+x*4+3]>16)xc[x]++;
        globalColumns=Bands(xc,3);if(globalColumns.Count!=6)throw new Exception("Ambiguous staggered atlas column layout ("+globalColumns.Count+" bands: "+String.Join(";",globalColumns.ConvertAll(v=>v[0]+","+v[1]))+"): "+input);
        columnRows=new List<int[]>[6];for(int col=0;col<6;col++) {
          var localY=new int[src.Height];for(int y=0;y<src.Height;y++)for(int x=globalColumns[col][0];x<globalColumns[col][1];x++)if(rgba[y*stride+x*4+3]>16)localY[y]++;
          columnRows[col]=Bands(localY,3);if(columnRows[col].Count!=4)throw new Exception("Expected 4 separated parts in column "+col+": "+input);
        }
      }
      var rectangles=new List<object>();
      using(var g=Graphics.FromImage(dst)) {
        g.Clear(Color.Transparent);g.CompositingMode=CompositingMode.SourceCopy;g.InterpolationMode=InterpolationMode.HighQualityBicubic;g.PixelOffsetMode=PixelOffsetMode.HighQuality;
        for(int row=0;row<4;row++) {
          int y0=0,y1=src.Height;List<int[]> columns;
          if(columnRows==null) {
            y0=rows[row][0];y1=rows[row][1];var xc=new int[src.Width];
            for(int y=y0;y<y1;y++)for(int x=0;x<src.Width;x++)if(rgba[y*stride+x*4+3]>16)xc[x]++;
            columns=Bands(xc,3);if(columns.Count!=6)throw new Exception("Expected 6 detached columns in row "+row+", got "+columns.Count+": "+input);
          } else columns=globalColumns;
          for(int col=0;col<6;col++) {
            if(columnRows!=null){y0=columnRows[col][row][0];y1=columnRows[col][row][1];}
            int x0=columns[col][0],x1=columns[col][1],minY=y1,maxY=y0,minX=x1,maxX=x0;
            for(int y=y0;y<y1;y++)for(int x=x0;x<x1;x++)if(rgba[y*stride+x*4+3]>16){minX=Math.Min(minX,x);maxX=Math.Max(maxX,x);minY=Math.Min(minY,y);maxY=Math.Max(maxY,y);}
            var crop=new Rectangle(Math.Max(0,minX-2),Math.Max(0,minY-2),Math.Min(src.Width,maxX+3)-Math.Max(0,minX-2),Math.Min(src.Height,maxY+3)-Math.Max(0,minY-2));
            double scale=Math.Min(224.0/crop.Width,224.0/crop.Height);int w=Math.Max(1,(int)Math.Round(crop.Width*scale)),h=Math.Max(1,(int)Math.Round(crop.Height*scale));
            var target=new Rectangle(col*256+(256-w)/2,row*256+(256-h)/2,w,h);g.DrawImage(src,target,crop,GraphicsUnit.Pixel);
            rectangles.Add(new { row=row,column=col,source=new int[]{crop.X,crop.Y,crop.Width,crop.Height},target=new int[]{target.X,target.Y,target.Width,target.Height} });
          }
        }
      }
      int borderMax=0;
      for(int row=0;row<4;row++)for(int col=0;col<6;col++)for(int p=0;p<256;p++){borderMax=Math.Max(borderMax,dst.GetPixel(col*256+p,row*256).A);borderMax=Math.Max(borderMax,dst.GetPixel(col*256+p,row*256+255).A);borderMax=Math.Max(borderMax,dst.GetPixel(col*256,row*256+p).A);borderMax=Math.Max(borderMax,dst.GetPixel(col*256+255,row*256+p).A);}
      if(borderMax!=0)throw new Exception("Formatted atlas border is not transparent: "+input);
      dst.Save(output,ImageFormat.Png);
      return new { source_width=src.Width,source_height=src.Height,alpha_min=0,alpha_max=maxAlpha,source_transparent_pixels=zero,width=1536,height=1024,cell_width=256,cell_height=256,padding=16,border_alpha_max=borderMax,layout_cuts=rectangles };
    }
  }
}
'@
$taskReferences = @([System.Drawing.Bitmap].Assembly.Location,[System.Drawing.Rectangle].Assembly.Location,'System.Runtime','System.Collections','System.Runtime.InteropServices',(Join-Path $PSHOME 'System.Private.Windows.GdiPlus.dll'),(Join-Path $PSHOME 'System.Private.Windows.Core.dll'))
if (-not ('ArtAtlasFormatter' -as [type])) { Add-Type -TypeDefinition $taskSource -ReferencedAssemblies $taskReferences -ErrorAction Stop }
$taskInputFull = (Resolve-Path -LiteralPath $InputPath).Path
$taskOutputFull = if ([System.IO.Path]::IsPathRooted($OutputPath)) { [System.IO.Path]::GetFullPath($OutputPath) } else { [System.IO.Path]::GetFullPath((Join-Path (Get-Location) $OutputPath)) }
if ($taskOutputFull -eq $taskInputFull) { throw 'Source and runtime output must be different files.' }
$taskResult = [ArtAtlasFormatter]::Format($taskInputFull,$taskOutputFull)
$taskResult | ConvertTo-Json -Depth 8 -Compress
