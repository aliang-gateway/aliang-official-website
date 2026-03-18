"use client";
import { useEffect, useRef } from "react";
import * as THREE from "three";

type TechBlip = {
  id: string;
  name: string;
  ring: "adopt" | "trial" | "assess" | "hold";
  x: number;
  y: number;
  relatedTags: string[];
  relatedKeywords: string[];
};

interface TechRadar3DProps {
  blips: TechBlip[];
  onBlipClick: (blip: TechBlip, buttonElement: HTMLButtonElement) => void;
}

export default function TechRadar3D({ blips, onBlipClick }: TechRadar3DProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    if (!canvasRef.current || !containerRef.current) return;

    const container = containerRef.current;
    const canvas = canvasRef.current;

    const scene = new THREE.Scene();
    const camera = new THREE.PerspectiveCamera(75, 1, 0.1, 1000);
    camera.position.z = 2;

    const renderer = new THREE.WebGLRenderer({ canvas, alpha: true, antialias: true });

    const ringColors = {
      hold: new THREE.Color("#64748b"),
      assess: new THREE.Color("#f59e0b"),
      trial: new THREE.Color("#3b82f6"),
      adopt: new THREE.Color("#10b981")
    };

    const rings: THREE.Points[] = [];
    const radii = [0.9, 0.68, 0.46, 0.24];
    const ringNames = ["hold", "assess", "trial", "adopt"] as const;

    radii.forEach((radius, i) => {
      const particles = 48; // Reduced from 80 for lower density
      const positions = new Float32Array(particles * 3);
      const colors = new Float32Array(particles * 3);
      const color = ringColors[ringNames[i]];

      for (let j = 0; j < particles; j++) {
        const angle = (j / particles) * Math.PI * 2;
        const wobble = Math.sin(angle * 3 + i) * 0.015;
        const randomOffset = (Math.random() - 0.5) * 0.01;
        positions[j * 3] = Math.cos(angle) * (radius + wobble + randomOffset);
        positions[j * 3 + 1] = Math.sin(angle) * (radius + wobble + randomOffset);
        positions[j * 3 + 2] = (Math.random() - 1.5) * 0.02;

        const brightness = 0.7 + Math.random() * 0.3;
        colors[j * 3] = color.r * brightness;
        colors[j * 3 + 1] = color.g * brightness;
        colors[j * 3 + 2] = color.b * brightness;
      }

      const geometry = new THREE.BufferGeometry();
      geometry.setAttribute("position", new THREE.BufferAttribute(positions, 3));
      geometry.setAttribute("color", new THREE.BufferAttribute(colors, 3));

      const material = new THREE.PointsMaterial({
        size: 0.022,
        vertexColors: true,
        transparent: true,
        opacity: 0.85,
        blending: THREE.AdditiveBlending
      });

      const points = new THREE.Points(geometry, material);
      scene.add(points);
      rings.push(points);
    });

    const lineMaterial = new THREE.LineBasicMaterial({ color: 0x444444, transparent: true, opacity: 0.3 });
    const vLineGeo = new THREE.BufferGeometry().setFromPoints([new THREE.Vector3(0, -1, 0), new THREE.Vector3(0, 1, 0)]);
    scene.add(new THREE.Line(vLineGeo, lineMaterial));
    const hLineGeo = new THREE.BufferGeometry().setFromPoints([new THREE.Vector3(-1, 0, 0), new THREE.Vector3(1, 0, 0)]);
    scene.add(new THREE.Line(hLineGeo, lineMaterial));

    const sweepLines: THREE.Line[] = [];
    const sweepCount = 6;
    for (let i = 0; i < sweepCount; i++) {
      const angle = (i / sweepCount) * Math.PI * 2;
      const points = [
        new THREE.Vector3(0, 0, 0),
        new THREE.Vector3(Math.cos(angle) * 0.95, Math.sin(angle) * 0.95, 0)
      ];
      const geo = new THREE.BufferGeometry().setFromPoints(points);
      const mat = new THREE.LineBasicMaterial({
        color: 0x10b981,
        transparent: true,
        opacity: 0.15
      });
      const line = new THREE.Line(geo, mat);
      scene.add(line);
      sweepLines.push(line);
    }

    const innerGlowGeo = new THREE.CircleGeometry(0.12, 64);
    const innerGlowMat = new THREE.MeshBasicMaterial({
      color: 0x10b981,
      transparent: true,
      opacity: 0.4
    });
    const innerGlow = new THREE.Mesh(innerGlowGeo, innerGlowMat);
    scene.add(innerGlow);

    const outerGlowGeo = new THREE.CircleGeometry(0.25, 64);
    const outerGlowMat = new THREE.MeshBasicMaterial({
      color: 0x3b82f6,
      transparent: true,
      opacity: 0.15
    });
    const outerGlow = new THREE.Mesh(outerGlowGeo, outerGlowMat);
    scene.add(outerGlow);

    const coreGlowGeo = new THREE.CircleGeometry(0.04, 32);
    const coreGlowMat = new THREE.MeshBasicMaterial({
      color: 0x10b981,
      transparent: true,
      opacity: 0.9
    });
    const coreGlow = new THREE.Mesh(coreGlowGeo, coreGlowMat);
    scene.add(coreGlow);

    const sparkleGeo = new THREE.BufferGeometry();
    const sparkleCount = 24;
    const sparklePos = new Float32Array(sparkleCount * 3);
    for (let i = 0; i < sparkleCount; i++) {
      const angle = Math.random() * Math.PI * 2;
      const r = 0.15 + Math.random() * 0.75;
      sparklePos[i * 3] = Math.cos(angle) * r;
      sparklePos[i * 3 + 1] = Math.sin(angle) * r;
      sparklePos[i * 3 + 2] = (Math.random() - 0.5) * 0.05;
    }
    sparkleGeo.setAttribute("position", new THREE.BufferAttribute(sparklePos, 3));
    const sparkleMat = new THREE.PointsMaterial({
      color: 0x10b981,
      size: 0.012,
      transparent: true,
      opacity: 0.6,
      blending: THREE.AdditiveBlending
    });
    const sparkles = new THREE.Points(sparkleGeo, sparkleMat);
    scene.add(sparkles);

    const floatingGeo = new THREE.BufferGeometry();
    const floatingCount = 18;
    const floatingPos = new Float32Array(floatingCount * 3);
    for (let i = 0; i < floatingCount; i++) {
      const angle = Math.random() * Math.PI * 2;
      const r = 0.25 + Math.random() * 0.65;
      floatingPos[i * 3] = Math.cos(angle) * r;
      floatingPos[i * 3 + 1] = Math.sin(angle) * r;
      floatingPos[i * 3 + 2] = (Math.random() - 0.5) * 0.05;
    }
    floatingGeo.setAttribute("position", new THREE.BufferAttribute(floatingPos, 3));
    const floatingMat = new THREE.PointsMaterial({
      color: 0x3b82f6,
      size: 0.015,
      transparent: true,
      opacity: 0.4,
      blending: THREE.AdditiveBlending
    });
    const floating = new THREE.Points(floatingGeo, floatingMat);
    scene.add(floating);

    function handleResize() {
      const width = container.clientWidth;
      const height = container.clientHeight;
      renderer.setSize(width, height);
      camera.aspect = width / height;
      camera.updateProjectionMatrix();
    }
    handleResize();

    const resizeObserver = new ResizeObserver(handleResize);
    resizeObserver.observe(container);

    let mouseX = 0;
    let mouseY = 0;
    let targetX = 0;
    let targetY = 0;

    function handleMouseMove(e: MouseEvent) {
      const rect = container.getBoundingClientRect();
      mouseX = ((e.clientX - rect.left) / rect.width - 0.5) * 0.08;
      mouseY = ((e.clientY - rect.top) / rect.height - 0.5) * 0.08;
    }
    container.addEventListener('mousemove', handleMouseMove);

    function handleMouseLeave() {
      mouseX = 0;
      mouseY = 0;
    }
    container.addEventListener('mouseleave', handleMouseLeave);

    let animationId: number;
    let time = 0;

    function animate() {
      animationId = requestAnimationFrame(animate);
      time += 0.01;

      targetX += (mouseX - targetX) * 0.04;
      targetY += (mouseY - targetY) * 0.04;
      camera.position.x = targetX;
      camera.position.y = targetY;
      camera.lookAt(0, 0, 0);

      rings.forEach((ring, i) => {
        ring.rotation.z = time * (0.08 + i * 0.04);
        const mat = ring.material as THREE.PointsMaterial;
        mat.opacity = 0.7 + Math.sin(time * 2 + i) * 0.2;
      });

      sweepLines.forEach((line, i) => {
        line.rotation.z = time * 0.3 + i * (Math.PI * 2 / sweepCount);
        const mat = line.material as THREE.LineBasicMaterial;
        mat.opacity = 0.1 + Math.sin(time * 3 + i) * 0.1;
      });

      sparkles.rotation.z = -time * 0.05;
      const sparkleMat = sparkles.material as THREE.PointsMaterial;
      sparkleMat.opacity = 0.4 + Math.sin(time * 4) * 0.3;

      floating.rotation.z = -time * 0.03;
      const floatMat = floating.material as THREE.PointsMaterial;
      floatMat.opacity = 0.25 + Math.sin(time * 1.5) * 0.15;

      innerGlow.scale.setScalar(1 + Math.sin(time * 2) * 0.2);
      outerGlow.scale.setScalar(1 + Math.sin(time * 1.5) * 0.15);
      coreGlow.scale.setScalar(1 + Math.sin(time * 4) * 0.3);

      const innerMat = innerGlow.material as THREE.MeshBasicMaterial;
      innerMat.opacity = 0.3 + Math.sin(time * 2) * 0.15;

      renderer.render(scene, camera);
    }
    animate();

    return () => {
      cancelAnimationFrame(animationId);
      resizeObserver.disconnect();
      container.removeEventListener('mousemove', handleMouseMove);
      container.removeEventListener('mouseleave', handleMouseLeave);
      rings.forEach(ring => { ring.geometry.dispose(); (ring.material as THREE.Material).dispose(); });
      sweepLines.forEach(line => { line.geometry.dispose(); (line.material as THREE.Material).dispose(); });
      sparkles.geometry.dispose(); (sparkles.material as THREE.Material).dispose();
      floating.geometry.dispose(); (floating.material as THREE.Material).dispose();
      renderer.dispose();
    };
  }, []);

  return (
    <div className="tech-radar-3d-wrapper">
      <div ref={containerRef} className="tech-radar-3d-container">
        <canvas ref={canvasRef} className="tech-radar-3d-canvas" />
        {blips.map((blip) => (
          <button
            key={blip.id}
            type="button"
            className={`blip blip-${blip.ring}`}
            style={{ left: `${blip.x}%`, top: `${blip.y}%` }}
            aria-label={`查看 ${blip.name} 相关博客`}
            title={blip.name}
            onClick={(e) => onBlipClick(blip, e.currentTarget)}
          >
            {blip.name.slice(0, 2)}
          </button>
        ))}
      </div>
    </div>
  );
}
